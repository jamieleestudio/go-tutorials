package gateway_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/jamieleestudio/go-tutorials/architecture/microservices/gateway"
	orderapp "github.com/jamieleestudio/go-tutorials/architecture/microservices/order/app"
	paymentapp "github.com/jamieleestudio/go-tutorials/architecture/microservices/payment/app"
	productapp "github.com/jamieleestudio/go-tutorials/architecture/microservices/product/app"
)

// ---- 端到端测试 ----
//
// 在一个测试进程里拉起 4 个真实组件 —— product（REST+gRPC）、payment（REST+gRPC）、
// order（REST + gRPC 客户端 + 熔断）、网关 —— 走真实 TCP gRPC + 真实 HTTP，
// 只有网关入口换成 httptest。零外部设施（Nacos/Zipkin 均不需要）。

func init() {
	gin.SetMode(gin.TestMode)
}

// freeAddr 分配一个空闲的 127.0.0.1 端口（存在微小竞态，教学测试可接受）。
func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("alloc port: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

// waitForReady 轮询 TCP 端口直到可连接（等服务启动）。
func waitForReady(t *testing.T, addr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("service at %s not ready within %s", addr, timeout)
}

// startRealStack 在测试进程内拉起 3 个服务（真实 TCP gRPC）+ httptest 网关。
func startRealStack(t *testing.T) *httptest.Server {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	suffix := strings.ReplaceAll(t.Name(), "/", "_")

	productHTTP, productGRPC := freeAddr(t), freeAddr(t)
	paymentHTTP, paymentGRPC := freeAddr(t), freeAddr(t)
	orderHTTP := freeAddr(t)

	go func() {
		_ = productapp.Run(ctx, productapp.Config{
			HTTPAddr: productHTTP, GRPCAddr: productGRPC,
			DSN:      "sqlite://file:e2e_product_" + suffix + "?mode=memory&cache=shared",
			LogLevel: "silent",
		})
	}()
	go func() {
		_ = paymentapp.Run(ctx, paymentapp.Config{
			HTTPAddr: paymentHTTP, GRPCAddr: paymentGRPC,
			DSN:      "sqlite://file:e2e_payment_" + suffix + "?mode=memory&cache=shared",
			LogLevel: "silent",
		})
	}()
	go func() {
		_ = orderapp.Run(ctx, orderapp.Config{
			HTTPAddr:        orderHTTP,
			ProductGRPCAddr: productGRPC,
			PaymentGRPCAddr: paymentGRPC,
			DSN:             "sqlite://file:e2e_order_" + suffix + "?mode=memory&cache=shared",
			LogLevel:        "silent",
		})
	}()

	for _, addr := range []string{productHTTP, productGRPC, paymentHTTP, paymentGRPC, orderHTTP} {
		waitForReady(t, addr, 15*time.Second)
	}

	stack := httptest.NewServer(gateway.New(gateway.Routes{
		OrderURL:   "http://" + orderHTTP,
		PaymentURL: "http://" + paymentHTTP,
		ProductURL: "http://" + productHTTP,
	}))
	t.Cleanup(func() { stack.Close() })
	return stack
}

// doJSON 执行请求并断言状态码，返回 JSON 对象。
func doJSON(t *testing.T, client *http.Client, method, url string, body any, wantStatus int) map[string]any {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	out := map[string]any{}
	_ = json.NewDecoder(resp.Body).Decode(&out) // 空响应体合法（如 pay 返回 {"message":"ok"}）
	if resp.StatusCode != wantStatus {
		t.Fatalf("%s %s: status = %d, want %d (body: %v)", method, url, resp.StatusCode, wantStatus, out)
	}
	return out
}

// paymentsList GET /api/payments（数组响应）。
func paymentsList(t *testing.T, client *http.Client, baseURL string) []map[string]any {
	t.Helper()
	resp, err := client.Get(baseURL + "/api/payments")
	if err != nil {
		t.Fatalf("list payments: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode payments: %v", err)
	}
	return out
}

func newClient() *http.Client { return &http.Client{Timeout: 10 * time.Second} }

// TestGatewayEndToEnd 全链路：商品 → 下单 → 支付（幂等）→ 取消（退款）。
func TestGatewayEndToEnd(t *testing.T) {
	stack := startRealStack(t)
	client := newClient()

	// 1. 经网关创建商品
	product := doJSON(t, client, http.MethodPost, stack.URL+"/api/products",
		map[string]any{"name": "Laptop", "price": "999.99", "stock": 10}, http.StatusCreated)
	productID, _ := product["productId"].(string)
	if productID == "" {
		t.Fatalf("no productId: %v", product)
	}

	// 2. 经网关下单
	order := doJSON(t, client, http.MethodPost, stack.URL+"/api/orders",
		map[string]any{
			"customerId": "customer-1",
			"lines":      []map[string]any{{"productId": productID, "quantity": 2}},
		}, http.StatusCreated)
	orderID, _ := order["orderId"].(string)
	if order["status"] != "CREATED" || orderID == "" {
		t.Fatalf("unexpected order: %v", order)
	}
	if order["totalAmount"] != "1999.98" {
		t.Fatalf("total = %v, want 1999.98", order["totalAmount"])
	}

	// 3. 经网关支付 → PAID
	doJSON(t, client, http.MethodPost, stack.URL+"/api/orders/"+orderID+"/pay", nil, http.StatusOK)
	paid := doJSON(t, client, http.MethodGet, stack.URL+"/api/orders/"+orderID, nil, http.StatusOK)
	if paid["status"] != "PAID" {
		t.Fatalf("status = %v, want PAID", paid["status"])
	}

	// 4. 重复支付被状态机拒绝（PAID → PAID 不允许，忠实 Java 语义；
	// payment 侧的 Pay RPC 幂等由 payment 模块的 rpc 测试覆盖）
	out := doJSON(t, client, http.MethodPost, stack.URL+"/api/orders/"+orderID+"/pay", nil, http.StatusBadRequest)
	if out["errorCode"] != "INVALID_TRANSITION" {
		t.Fatalf("errorCode = %v, want INVALID_TRANSITION", out["errorCode"])
	}

	// 5. 经网关取消已支付订单 → 触发退款
	doJSON(t, client, http.MethodPost, stack.URL+"/api/orders/"+orderID+"/cancel",
		map[string]any{"reason": "changed mind"}, http.StatusOK)
	cancelled := doJSON(t, client, http.MethodGet, stack.URL+"/api/orders/"+orderID, nil, http.StatusOK)
	if cancelled["status"] != "CANCELLED" {
		t.Fatalf("status = %v, want CANCELLED", cancelled["status"])
	}

	// 6. 退款生效：支付单 REFUNDED
	paymentList := paymentsList(t, client, stack.URL)
	if len(paymentList) != 1 || paymentList[0]["status"] != "REFUNDED" {
		t.Fatalf("payments after refund: %v", paymentList)
	}
}

// TestGatewayFallbackWhenPaymentDown 熔断降级：
// payment 的 gRPC 地址指向空端口 → Pay 失败 → order 侧 fallback 返回
// PAYMENT_SERVICE_UNAVAILABLE（对应 Java Feign + Resilience4j fallback）。
func TestGatewayFallbackWhenPaymentDown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	suffix := strings.ReplaceAll(t.Name(), "/", "_")

	productHTTP, productGRPC := freeAddr(t), freeAddr(t)
	orderHTTP := freeAddr(t)
	deadPaymentGRPC := freeAddr(t) // 没有任何服务监听

	go func() {
		_ = productapp.Run(ctx, productapp.Config{
			HTTPAddr: productHTTP, GRPCAddr: productGRPC,
			DSN:      "sqlite://file:e2e_product_" + suffix + "?mode=memory&cache=shared",
			LogLevel: "silent",
		})
	}()
	go func() {
		_ = orderapp.Run(ctx, orderapp.Config{
			HTTPAddr:        orderHTTP,
			ProductGRPCAddr: productGRPC,
			PaymentGRPCAddr: deadPaymentGRPC,
			DSN:             "sqlite://file:e2e_order_" + suffix + "?mode=memory&cache=shared",
			LogLevel:        "silent",
		})
	}()
	waitForReady(t, productHTTP, 15*time.Second)
	waitForReady(t, orderHTTP, 15*time.Second)

	stack := httptest.NewServer(gateway.New(gateway.Routes{
		OrderURL:   "http://" + orderHTTP,
		PaymentURL: "http://127.0.0.1:1", // 不可达
		ProductURL: "http://" + productHTTP,
	}))
	t.Cleanup(func() { stack.Close() })
	client := newClient()

	product := doJSON(t, client, http.MethodPost, stack.URL+"/api/products",
		map[string]any{"name": "SSD", "price": "199.00", "stock": 5}, http.StatusCreated)
	productID, _ := product["productId"].(string)

	order := doJSON(t, client, http.MethodPost, stack.URL+"/api/orders",
		map[string]any{
			"customerId": "customer-1",
			"lines":      []map[string]any{{"productId": productID, "quantity": 1}},
		}, http.StatusCreated)
	orderID, _ := order["orderId"].(string)

	// 支付失败 → fallback（Feign fallback 对应物）
	out := doJSON(t, client, http.MethodPost, stack.URL+"/api/orders/"+orderID+"/pay", nil, http.StatusBadRequest)
	if out["errorCode"] != "PAYMENT_SERVICE_UNAVAILABLE" {
		t.Fatalf("errorCode = %v, want PAYMENT_SERVICE_UNAVAILABLE", out["errorCode"])
	}

	// 订单仍是 CREATED（未误标记）
	got := doJSON(t, client, http.MethodGet, stack.URL+"/api/orders/"+orderID, nil, http.StatusOK)
	if got["status"] != "CREATED" {
		t.Fatalf("status = %v, want CREATED", got["status"])
	}
}

// TestGatewayFallbackWhenProductDown product 不可达 → 创建订单失败
// → PRODUCT_SERVICE_UNAVAILABLE（order 应用层不感知远程细节）。
func TestGatewayFallbackWhenProductDown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	suffix := strings.ReplaceAll(t.Name(), "/", "_")

	paymentHTTP, paymentGRPC := freeAddr(t), freeAddr(t)
	orderHTTP := freeAddr(t)
	deadProductGRPC := freeAddr(t)

	go func() {
		_ = paymentapp.Run(ctx, paymentapp.Config{
			HTTPAddr: paymentHTTP, GRPCAddr: paymentGRPC,
			DSN:      "sqlite://file:e2e_payment_" + suffix + "?mode=memory&cache=shared",
			LogLevel: "silent",
		})
	}()
	go func() {
		_ = orderapp.Run(ctx, orderapp.Config{
			HTTPAddr:        orderHTTP,
			ProductGRPCAddr: deadProductGRPC,
			PaymentGRPCAddr: paymentGRPC,
			DSN:             "sqlite://file:e2e_order_" + suffix + "?mode=memory&cache=shared",
			LogLevel:        "silent",
		})
	}()
	waitForReady(t, orderHTTP, 15*time.Second)

	stack := httptest.NewServer(gateway.New(gateway.Routes{
		OrderURL:   "http://" + orderHTTP,
		PaymentURL: "http://" + paymentHTTP,
		ProductURL: "http://127.0.0.1:1",
	}))
	t.Cleanup(func() { stack.Close() })
	client := newClient()

	out := doJSON(t, client, http.MethodPost, stack.URL+"/api/orders",
		map[string]any{
			"customerId": "customer-1",
			"lines":      []map[string]any{{"productId": "p-x", "quantity": 1}},
		}, http.StatusBadRequest)
	if out["errorCode"] != "PRODUCT_SERVICE_UNAVAILABLE" {
		t.Fatalf("errorCode = %v, want PRODUCT_SERVICE_UNAVAILABLE", out["errorCode"])
	}
}

// TestGatewayNotFoundViaUpstream 网关透传上游 404（错误契约经网关原样返回）。
func TestGatewayNotFoundViaUpstream(t *testing.T) {
	stack := startRealStack(t)
	client := newClient()

	out := doJSON(t, client, http.MethodGet, stack.URL+"/api/orders/no-such-order", nil, http.StatusNotFound)
	if out["errorCode"] != "ORDER_NOT_FOUND" {
		t.Fatalf("errorCode = %v, want ORDER_NOT_FOUND", out["errorCode"])
	}
}
