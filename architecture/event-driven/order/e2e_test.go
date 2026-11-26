// 端到端测试 —— Go 增强版（Java 版的 Saga 集成验证依赖 testcontainers/Kafka；
// Go 教学版用 HTTP webhook 投递，可以在进程内起【真实 TCP】的三服务栈：
// 真 gin HTTP server + 真 gRPC server + 真 SQLite 库 + 真 Outbox Relay）。
//
// 覆盖四个场景：
//  1. 全链路 Saga：下单 → 支付 + 扣减 → 双到齐 CONFIRMED（最终一致性）
//  2. 缺货补偿：OUT_OF_STOCK → InventoryDeductFailed → 订单 CANCELLED（零部分扣减）
//  3. 取消补偿：CREATED → cancel → OrderCancelled → 库存凭记账回补
//  4. 重放幂等：同一 eventId 重复投递 → 去重 → 只处理一次（at-least-once 的兜底）
package order_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	orderapp "github.com/jamieleestudio/go-tutorials/architecture/event-driven/order/app"
	paymentapp "github.com/jamieleestudio/go-tutorials/architecture/event-driven/payment/app"
	productapp "github.com/jamieleestudio/go-tutorials/architecture/event-driven/product/app"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared/event"
)

const (
	e2eRelayInterval = 50 * time.Millisecond
	e2eTimeout       = 10 * time.Second
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

// ---- 进程内装配 helper（对应 @SpringBootTest 的随机端口启动）----

type runningServer struct {
	restBase string
	grpcAddr string // 仅 product 有

	// subscribers 是 Relay 的投递表（HTTPDeliver 闭包引用同一 map ——
	// 服务全部起来后 rewire 改内容即生效）。
	subscribers map[string][]string
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

// startProduct 装配 product：REST + gRPC + Relay（订阅者表可通过 rewire 接线）。
func startProduct(t *testing.T, name string) *runningServer {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	subscribers := map[string][]string{event.TopicProductEvents: nil}
	cfg := productapp.Config{
		DSN:              fmt.Sprintf("sqlite://file:eda-e2e-product-%s?mode=memory&cache=shared", name),
		LogLevel:         "silent",
		RelayInterval:    e2eRelayInterval,
		EventSubscribers: subscribers,
	}
	repos, err := productapp.NewRepositories(cfg)
	if err != nil {
		t.Fatalf("product repos: %v", err)
	}
	svc := productapp.NewAppService(repos)

	restBase, stopHTTP := serveHTTP(t, productapp.NewHTTPServer(svc))

	// gRPC：真 TCP listener（order 的同步目录读打到这里）
	srv := productapp.NewGRPCServer(svc)
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("product grpc listen: %v", err)
	}
	go func() { _ = srv.Serve(lis) }()
	go productapp.NewRelay(repos, cfg).Run(ctx)

	t.Cleanup(func() {
		cancel()
		stopHTTP()
		srv.GracefulStop()
	})
	return &runningServer{restBase: restBase, grpcAddr: lis.Addr().String(), subscribers: subscribers}
}

func startPayment(t *testing.T, name string) *runningServer {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	subscribers := map[string][]string{event.TopicPaymentEvents: nil}
	cfg := paymentapp.Config{
		DSN:              fmt.Sprintf("sqlite://file:eda-e2e-payment-%s?mode=memory&cache=shared", name),
		LogLevel:         "silent",
		RelayInterval:    e2eRelayInterval,
		EventSubscribers: subscribers,
	}
	repos, err := paymentapp.NewRepositories(cfg)
	if err != nil {
		t.Fatalf("payment repos: %v", err)
	}
	svc := paymentapp.NewAppService(repos)
	restBase, stopHTTP := serveHTTP(t, paymentapp.NewHTTPServer(svc))
	go paymentapp.NewRelay(repos, cfg).Run(ctx)

	t.Cleanup(func() {
		cancel()
		stopHTTP()
	})
	return &runningServer{restBase: restBase, subscribers: subscribers}
}

func startOrder(t *testing.T, name string, productGRPCAddr string) *runningServer {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	subscribers := map[string][]string{event.TopicOrderEvents: nil}
	cfg := orderapp.Config{
		DSN:              fmt.Sprintf("sqlite://file:eda-e2e-order-%s?mode=memory&cache=shared", name),
		LogLevel:         "silent",
		ProductGRPCAddr:  productGRPCAddr,
		RelayInterval:    e2eRelayInterval,
		EventSubscribers: subscribers,
	}
	repos, err := orderapp.NewRepositories(cfg)
	if err != nil {
		t.Fatalf("order repos: %v", err)
	}
	productClient, err := orderapp.NewProductClient(productGRPCAddr)
	if err != nil {
		t.Fatalf("order product client: %v", err)
	}
	svc := orderapp.NewAppService(repos, productClient)
	restBase, stopHTTP := serveHTTP(t, orderapp.NewHTTPServer(svc))
	go orderapp.NewRelay(repos, cfg).Run(ctx)

	t.Cleanup(func() {
		cancel()
		stopHTTP()
		_ = productClient.Close()
	})
	return &runningServer{restBase: restBase, subscribers: subscribers}
}

// rewire 三服务的地址都是随机分配的，只能在全部起来后接线：
// order-events → payment + product；payment-events / product-events → order。
// HTTPDeliver 闭包引用同一 map，改内容即生效（下一轮 Relay 投递即用新表）。
func rewire(svc *runningServer, topic string, urls []string) {
	svc.subscribers[topic] = urls
}

// ---- HTTP helper ----

type productResponse struct {
	ProductID string `json:"productId"`
	Name      string `json:"name"`
	Price     string `json:"price"`
	Stock     int    `json:"stock"`
}

type orderResponse struct {
	OrderID           string `json:"orderId"`
	CustomerID        string `json:"customerId"`
	Status            string `json:"status"`
	TotalAmount       string `json:"totalAmount"`
	PaymentSucceeded  bool   `json:"paymentSucceeded"`
	InventoryDeducted bool   `json:"inventoryDeducted"`
}

func doJSON(t *testing.T, method, url string, body any, out any) int {
	t.Helper()
	raw := []byte(nil)
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if out != nil && resp.StatusCode < 300 {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode %s %s: %v", method, url, err)
		}
	}
	return resp.StatusCode
}

func postEvent(t *testing.T, base string, envelope event.Envelope) int {
	t.Helper()
	return doJSON(t, http.MethodPost, base+"/internal/events", envelope, nil)
}

// waitForReady 轮询探活直到服务可用。
//
// probePath 必须是该服务【真实存在】的路由——三个服务的路由表各不相同：
// product 有 /api/products，order 只有 /api/orders（payment 无业务端点，
// 不需要探活）。探错服务轮询一个不存在的路由只会永远 404。
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

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(e2eTimeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for: %s", what)
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

func getProduct(t *testing.T, base, productID string) productResponse {
	t.Helper()
	var p productResponse
	code := doJSON(t, http.MethodGet, base+"/api/products/"+productID, nil, &p)
	if code != http.StatusOK {
		t.Fatalf("get product: status %d", code)
	}
	return p
}

// wireFullSaga 接通全量 Saga 拓扑（order → payment+product → order）。
func wireFullSaga(order, payment, product *runningServer) {
	rewire(order, event.TopicOrderEvents, []string{
		payment.restBase + "/internal/events",
		product.restBase + "/internal/events",
	})
	rewire(payment, event.TopicPaymentEvents, []string{order.restBase + "/internal/events"})
	rewire(product, event.TopicProductEvents, []string{order.restBase + "/internal/events"})
}

// ---- 场景 1：全链路 Saga → CONFIRMED ----

func TestFullSagaFlowConfirmsOrder(t *testing.T) {
	product := startProduct(t, "full")
	payment := startPayment(t, "full")
	order := startOrder(t, "full", product.grpcAddr)
	wireFullSaga(order, payment, product)

	waitForReady(t, product.restBase, "/api/products")
	waitForReady(t, order.restBase, "/api/orders")

	p1 := seedProduct(t, product.restBase, "Laptop", "100.00", 10)

	created := createOrder(t, order.restBase, "c-1", p1, 2)
	if created.Status != "CREATED" {
		t.Fatalf("fresh order status = %s, want CREATED", created.Status)
	}

	// Saga 收敛：支付与扣减都是异步的，双到齐 → CONFIRMED（最终一致性时刻）
	eventually(t, "order CONFIRMED", func() bool {
		o := getOrder(t, order.restBase, created.OrderID)
		return o.Status == "CONFIRMED" && o.PaymentSucceeded && o.InventoryDeducted
	})
	final := getOrder(t, order.restBase, created.OrderID)
	if final.TotalAmount != "200.00" {
		t.Fatalf("totalAmount = %s, want 200.00", final.TotalAmount)
	}
	if got := getProduct(t, product.restBase, p1).Stock; got != 8 {
		t.Fatalf("stock = %d, want 8 (10 - 2)", got)
	}
}

// ---- 场景 2：缺货 → InventoryDeductFailed → 订单取消补偿（零部分扣减） ----

func TestOutOfStockCancelsOrder(t *testing.T) {
	product := startProduct(t, "oos")
	payment := startPayment(t, "oos")
	order := startOrder(t, "oos", product.grpcAddr)
	wireFullSaga(order, payment, product)

	waitForReady(t, product.restBase, "/api/products")
	waitForReady(t, order.restBase, "/api/orders")

	p1 := seedProduct(t, product.restBase, "Limited", "10.00", 1)

	created := createOrder(t, order.restBase, "c-2", p1, 2)

	eventually(t, "order CANCELLED by out-of-stock", func() bool {
		return getOrder(t, order.restBase, created.OrderID).Status == "CANCELLED"
	})
	// 预校验语义：失败时零部分扣减
	if got := getProduct(t, product.restBase, p1).Stock; got != 1 {
		t.Fatalf("stock = %d, want 1 (no partial deduction)", got)
	}
}

// ---- 场景 3：取消补偿 —— CREATED → cancel → 库存凭记账回补 ----
//
// 装配里 order 只订阅 product（payment 缺席）：订单永远等不到支付成功，
// 稳定停留在 CREATED —— 为 cancel 制造确定性时序窗口。

func TestCancelRestocksInventory(t *testing.T) {
	product := startProduct(t, "cancel")
	order := startOrder(t, "cancel", product.grpcAddr)
	rewire(order, event.TopicOrderEvents, []string{product.restBase + "/internal/events"})
	rewire(product, event.TopicProductEvents, []string{order.restBase + "/internal/events"})

	waitForReady(t, product.restBase, "/api/products")
	waitForReady(t, order.restBase, "/api/orders")

	p1 := seedProduct(t, product.restBase, "Widget", "5.00", 3)

	created := createOrder(t, order.restBase, "c-3", p1, 1)
	eventually(t, "inventory deducted (stock 3→2)", func() bool {
		return getProduct(t, product.restBase, p1).Stock == 2
	})

	code := doJSON(t, http.MethodPost, order.restBase+"/api/orders/"+created.OrderID+"/cancel",
		map[string]any{"reason": "changed my mind"}, nil)
	if code != http.StatusOK {
		t.Fatalf("cancel: status %d", code)
	}

	eventually(t, "order CANCELLED", func() bool {
		return getOrder(t, order.restBase, created.OrderID).Status == "CANCELLED"
	})
	eventually(t, "inventory restocked via compensation (stock 2→3)", func() bool {
		return getProduct(t, product.restBase, p1).Stock == 3
	})
}

// ---- 场景 4：重放幂等 —— 同一 eventId 重复投递 → 去重 → 只扣一次 ----

func TestDuplicateEventIsDedupedByProcessedTable(t *testing.T) {
	product := startProduct(t, "dup")
	waitForReady(t, product.restBase, "/api/products")

	p1 := seedProduct(t, product.restBase, "DedupTarget", "1.00", 10)

	payload, err := json.Marshal(event.OrderCreatedEvent{
		OrderID: "order-dup",
		Lines: []event.OrderLine{{
			ProductID: p1, ProductName: "DedupTarget", Quantity: 4, UnitPrice: "1.00",
		}},
		TotalAmount: "4.00",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	envelope := event.Envelope{
		EventID:     "evt-dup-1",
		Type:        event.TypeOrderCreated,
		AggregateID: "order-dup",
		Topic:       event.TopicOrderEvents,
		Payload:     payload,
	}

	// 第一次投递：真正扣减
	if code := postEvent(t, product.restBase, envelope); code != http.StatusOK {
		t.Fatalf("first delivery: status %d", code)
	}
	if got := getProduct(t, product.restBase, p1).Stock; got != 6 {
		t.Fatalf("stock after first delivery = %d, want 6", got)
	}

	// at-least-once 重放：同一 eventId 再投 → processed_events 去重 → 只扣一次
	if code := postEvent(t, product.restBase, envelope); code != http.StatusOK {
		t.Fatalf("duplicate delivery must be acknowledged, status %d", code)
	}
	if got := getProduct(t, product.restBase, p1).Stock; got != 6 {
		t.Fatalf("stock after replay = %d, want 6 (deduped)", got)
	}
}
