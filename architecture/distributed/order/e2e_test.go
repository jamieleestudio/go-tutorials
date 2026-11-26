// 端到端测试 —— 进程内起【真实 TCP】的三服务栈：
// 真 gin HTTP server + 真 SQLite 库 + 真 HTTP RPC（含真实的超时/重试客户端）。
// 对应 Java 版依赖 3 个 Spring Boot 进程 + application.yml 固定地址的手工联调。
//
// 覆盖四个场景：
//  1. 全链路 pay：下单（远程查品）→ 支付（远程 RPC）→ PAID + payment SUCCESS；
//  2. 幂等重放：同一 orderId 的 pay RPC 重放 → 同一 paymentId（重试安全的根基）；
//  3. 取消退款：pay → cancel → 订单 CANCELLED + payment REFUNDED（远程退款）；
//  4. product RPC 契约：/rpc/products 的单查/批查/404。
package order_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	orderapp "github.com/jamieleestudio/go-tutorials/architecture/distributed/order/app"
	paymentapp "github.com/jamieleestudio/go-tutorials/architecture/distributed/payment/app"
	productapp "github.com/jamieleestudio/go-tutorials/architecture/distributed/product/app"
)

const e2eTimeout = 10 * time.Second

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

// ---- 进程内装配 helper（对应 @SpringBootTest 的随机端口启动）----

type runningServer struct {
	restBase string
}

func serveHTTP(t *testing.T, handler http.Handler) (string, func()) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = server.Serve(lis) }()
	base := "http://" + lis.Addr().String()
	stop := func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}
	return base, stop
}

// startProduct 装配 product：REST /api/products + RPC /rpc/products。
func startProduct(t *testing.T, name string) *runningServer {
	t.Helper()
	base, stop := serveHTTP(t, mustProductService(t, name))
	t.Cleanup(stop)
	return &runningServer{restBase: base}
}

// startPayment 装配 payment：REST /api/payments + RPC /rpc/payments。
func startPayment(t *testing.T, name string) *runningServer {
	t.Helper()
	base, stop := serveHTTP(t, mustPaymentService(t, name))
	t.Cleanup(stop)
	return &runningServer{restBase: base}
}

// startOrder 装配 order：REST /api/orders；跨服务客户端指向两个随机端口 ——
// P6 的"固定地址"在 e2e 里被替换成真实分配的地址（env 覆盖同一机制）。
func startOrder(t *testing.T, name, paymentBase, productBase string) *runningServer {
	t.Helper()
	cfg := orderapp.Config{
		DSN:               fmt.Sprintf("sqlite://file:dist-e2e-order-%s?mode=memory&cache=shared", name),
		LogLevel:          "silent",
		PaymentServiceURL: paymentBase,
		ProductServiceURL: productBase,
	}
	repos, err := orderapp.NewRepositories(cfg)
	if err != nil {
		t.Fatalf("order repos: %v", err)
	}
	publisher, _ := orderapp.NewPublisher()
	payments, products := orderapp.NewRPCClients(cfg)
	svc := orderapp.NewAppService(repos, payments, products, publisher)
	base, stop := serveHTTP(t, orderapp.NewHTTPServer(svc))
	t.Cleanup(stop)
	return &runningServer{restBase: base}
}

func mustProductService(t *testing.T, name string) http.Handler {
	t.Helper()
	cfg := productapp.Config{
		DSN:      fmt.Sprintf("sqlite://file:dist-e2e-product-%s?mode=memory&cache=shared", name),
		LogLevel: "silent",
	}
	repos, err := productapp.NewRepositories(cfg)
	if err != nil {
		t.Fatalf("product repos: %v", err)
	}
	return productapp.NewHTTPServer(productapp.NewAppService(repos))
}

func mustPaymentService(t *testing.T, name string) http.Handler {
	t.Helper()
	cfg := paymentapp.Config{
		DSN:      fmt.Sprintf("sqlite://file:dist-e2e-payment-%s?mode=memory&cache=shared", name),
		LogLevel: "silent",
	}
	repos, err := paymentapp.NewRepositories(cfg)
	if err != nil {
		t.Fatalf("payment repos: %v", err)
	}
	return paymentapp.NewHTTPServer(paymentapp.NewAppService(repos))
}

// ---- HTTP helper ----

type productResponse struct {
	ProductID string `json:"productId"`
	Name      string `json:"name"`
	Price     string `json:"price"`
	Stock     int    `json:"stock"`
}

type orderResponse struct {
	OrderID     string `json:"orderId"`
	CustomerID  string `json:"customerId"`
	Status      string `json:"status"`
	TotalAmount string `json:"totalAmount"`
}

type paymentResponse struct {
	PaymentID string `json:"paymentId"`
	OrderID   string `json:"orderId"`
	Amount    string `json:"amount"`
	Status    string `json:"status"`
}

type errorResponse struct {
	ErrorCode string `json:"errorCode"`
	Message   string `json:"message"`
}

func doJSON(t *testing.T, method, url string, body any, out any) int {
	t.Helper()
	raw := []byte(nil)
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	req, err := http.NewRequest(method, url, strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if out != nil {
		// 错误响应（4xx/5xx）也按 JSON 解码 —— error 契约 {errorCode, message} 同样要可断言。
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode %s %s: %v", method, url, err)
		}
	}
	return resp.StatusCode
}

// postText RPC 探针：POST query 参数请求，返回状态码与纯文本响应体
// （payment 的 pay 契约返回纯文本 paymentId，不是 JSON）。
func postText(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Post(url, "text/plain", nil)
	if err != nil {
		t.Fatalf("post %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		t.Fatalf("read %s: %v", url, err)
	}
	return resp.StatusCode, strings.TrimSpace(string(body))
}

// waitForReady 轮询探活直到服务可用。
// probePath 必须是该服务【真实存在】的路由：product 是 /api/products、
// payment 是 /api/payments、order 是 /api/orders（P5 的教训沿用）。
func waitForReady(t *testing.T, base, probePath string) {
	t.Helper()
	deadline := time.Now().Add(e2eTimeout)
	lastStatus := 0
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + probePath)
		if err == nil {
			_ = resp.Body.Close()
			lastStatus = resp.StatusCode
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("service %s%s never became ready: last status %d", base, probePath, lastStatus)
}

func seedProduct(t *testing.T, base, name, price string, stock int) string {
	t.Helper()
	var created productResponse
	code := doJSON(t, http.MethodPost, base+"/api/products",
		map[string]any{"name": name, "price": price, "stock": stock}, &created)
	if code != http.StatusCreated {
		t.Fatalf("seed product: status %d", code)
	}
	return created.ProductID
}

func createOrder(t *testing.T, base, customerID, productID string, qty int) orderResponse {
	t.Helper()
	var created orderResponse
	code := doJSON(t, http.MethodPost, base+"/api/orders", map[string]any{
		"customerId": customerID,
		"lines":      []map[string]any{{"productId": productID, "quantity": qty}},
	}, &created)
	if code != http.StatusCreated {
		t.Fatalf("create order: status %d", code)
	}
	return created
}

func getOrder(t *testing.T, base, orderID string) orderResponse {
	t.Helper()
	var order orderResponse
	code := doJSON(t, http.MethodGet, base+"/api/orders/"+orderID, nil, &order)
	if code != http.StatusOK {
		t.Fatalf("get order: status %d", code)
	}
	return order
}

func listPayments(t *testing.T, base string) []paymentResponse {
	t.Helper()
	var payments []paymentResponse
	code := doJSON(t, http.MethodGet, base+"/api/payments", nil, &payments)
	if code != http.StatusOK {
		t.Fatalf("list payments: status %d", code)
	}
	return payments
}

func paymentByOrder(t *testing.T, base, orderID string) paymentResponse {
	t.Helper()
	for _, p := range listPayments(t, base) {
		if p.OrderID == orderID {
			return p
		}
	}
	t.Fatalf("no payment for order %s", orderID)
	return paymentResponse{}
}

// ---- 场景 1：全链路 pay（远程查品 + 远程支付）→ PAID ----

func TestFullChainPayMarksOrderPaid(t *testing.T) {
	product := startProduct(t, "full")
	payment := startPayment(t, "full")
	order := startOrder(t, "full", payment.restBase, product.restBase)

	waitForReady(t, product.restBase, "/api/products")
	waitForReady(t, order.restBase, "/api/orders")

	p1 := seedProduct(t, product.restBase, "Laptop", "999.99", 10)

	// 下单会远程调用 product 服务校验商品（RPC 契约第一次生效）
	created := createOrder(t, order.restBase, "c-1", p1, 2)
	if created.Status != "CREATED" {
		t.Fatalf("fresh order status = %s, want CREATED", created.Status)
	}
	if created.TotalAmount != "1999.98" {
		t.Fatalf("totalAmount = %s, want 1999.98", created.TotalAmount)
	}

	// 支付会远程调用 payment 服务（RPC 契约第二次生效）
	if code := doJSON(t, http.MethodPost, order.restBase+"/api/orders/"+created.OrderID+"/pay", nil, nil); code != http.StatusOK {
		t.Fatalf("pay: status %d", code)
	}

	paid := getOrder(t, order.restBase, created.OrderID)
	if paid.Status != "PAID" {
		t.Fatalf("status after pay = %s, want PAID", paid.Status)
	}
	p := paymentByOrder(t, payment.restBase, created.OrderID)
	if p.Status != "SUCCESS" || p.Amount != "1999.98" {
		t.Fatalf("payment = %+v, want SUCCESS / 1999.98", p)
	}
}

// ---- 场景 2：幂等重放 —— 同一 orderId 的 pay RPC 重放 → 同一 paymentId ----

func TestPayIsIdempotentOnRpcReplay(t *testing.T) {
	product := startProduct(t, "idem")
	payment := startPayment(t, "idem")
	order := startOrder(t, "idem", payment.restBase, product.restBase)

	waitForReady(t, product.restBase, "/api/products")
	waitForReady(t, order.restBase, "/api/orders")

	p1 := seedProduct(t, product.restBase, "IdempotencyProbe", "50.00", 10)
	created := createOrder(t, order.restBase, "c-2", p1, 3)
	if code := doJSON(t, http.MethodPost, order.restBase+"/api/orders/"+created.OrderID+"/pay", nil, nil); code != http.StatusOK {
		t.Fatalf("pay: status %d", code)
	}
	first := paymentByOrder(t, payment.restBase, created.OrderID)

	// 直接对 payment 服务重放同一笔 pay RPC —— 模拟"网络层重试导致 POST 执行两次"的后果。
	// 幂等：返回同一 paymentId，不会产生第二笔支付。
	replayURL := fmt.Sprintf("%s/rpc/payments/pay?orderId=%s&amount=150.00", payment.restBase, created.OrderID)
	code, body := postText(t, replayURL)
	if code != http.StatusOK {
		t.Fatalf("replay pay: status %d", code)
	}
	if body != first.PaymentID {
		t.Fatalf("replay returned paymentId %q, want same %q (idempotency broken)", body, first.PaymentID)
	}
	if payments := listPayments(t, payment.restBase); len(payments) != 1 {
		t.Fatalf("payment count = %d, want 1 (replay must not create a second payment)", len(payments))
	}

	// 订单层面重复支付 → 状态机规则违约（PAID 不允许再 pay）
	if code := doJSON(t, http.MethodPost, order.restBase+"/api/orders/"+created.OrderID+"/pay", nil, nil); code != http.StatusBadRequest {
		t.Fatalf("double pay: status %d, want 400", code)
	}
}

// ---- 场景 3：取消退款 —— pay → cancel → 远程 refund ----

func TestCancelRefundsPayment(t *testing.T) {
	product := startProduct(t, "cancel")
	payment := startPayment(t, "cancel")
	order := startOrder(t, "cancel", payment.restBase, product.restBase)

	waitForReady(t, product.restBase, "/api/products")
	waitForReady(t, order.restBase, "/api/orders")

	p1 := seedProduct(t, product.restBase, "RefundProbe", "10.00", 5)
	created := createOrder(t, order.restBase, "c-3", p1, 1)
	if code := doJSON(t, http.MethodPost, order.restBase+"/api/orders/"+created.OrderID+"/pay", nil, nil); code != http.StatusOK {
		t.Fatalf("pay: status %d", code)
	}
	paid := paymentByOrder(t, payment.restBase, created.OrderID)

	if code := doJSON(t, http.MethodPost, order.restBase+"/api/orders/"+created.OrderID+"/cancel",
		map[string]any{"reason": "changed my mind"}, nil); code != http.StatusOK {
		t.Fatalf("cancel: status %d", code)
	}

	if got := getOrder(t, order.restBase, created.OrderID).Status; got != "CANCELLED" {
		t.Fatalf("status after cancel = %s, want CANCELLED", got)
	}
	// 取消编排远程调用了 refund：payment 状态推进到 REFUNDED
	var refunded paymentResponse
	code := doJSON(t, http.MethodGet, payment.restBase+"/api/payments/"+paid.PaymentID, nil, &refunded)
	if code != http.StatusOK {
		t.Fatalf("get payment: status %d", code)
	}
	if refunded.Status != "REFUNDED" {
		t.Fatalf("payment status after cancel = %s, want REFUNDED", refunded.Status)
	}
}

// ---- 场景 4：product RPC 契约（单查 / 批查 / 404）----

func TestProductRpcEndpoints(t *testing.T) {
	product := startProduct(t, "rpc")
	waitForReady(t, product.restBase, "/api/products")

	p1 := seedProduct(t, product.restBase, "RpcProbe", "42.00", 7)

	// 单查：JSON 字段名由契约包的 json tag 决定（对应 Java record 的 Jackson 协议）。
	// 注意 price 是 "42" 而非 "42.00"：Go 的 decimal 序列化会裁掉尾零（String()）；
	// 需要 fixed scale 的金额（如支付金额）在 money 边界用 StringFixed(2) 传递。
	var dto productResponse
	if code := doJSON(t, http.MethodGet, product.restBase+"/rpc/products/"+p1, nil, &dto); code != http.StatusOK {
		t.Fatalf("rpc get product: status %d", code)
	}
	if dto.ProductID != p1 || dto.Name != "RpcProbe" || dto.Price != "42" || dto.Stock != 7 {
		t.Fatalf("rpc product = %+v", dto)
	}

	// 批查：ids 逗号分隔；不存在的 ID 不报错，只返回找到的部分（与 Java 一致）
	var list []productResponse
	if code := doJSON(t, http.MethodGet, product.restBase+"/rpc/products?ids="+p1+",ghost", nil, &list); code != http.StatusOK {
		t.Fatalf("rpc get products: status %d", code)
	}
	if len(list) != 1 || list[0].ProductID != p1 {
		t.Fatalf("rpc products = %+v, want only %s", list, p1)
	}

	// 未知商品 → 404 PRODUCT_NOT_FOUND（error 契约与 REST 共用同一套）
	var errResp errorResponse
	code := doJSON(t, http.MethodGet, product.restBase+"/rpc/products/ghost", nil, &errResp)
	if code != http.StatusNotFound {
		t.Fatalf("rpc unknown product: status %d, want 404", code)
	}
	if errResp.ErrorCode != "PRODUCT_NOT_FOUND" {
		t.Fatalf("errorCode = %s, want PRODUCT_NOT_FOUND", errResp.ErrorCode)
	}
}
