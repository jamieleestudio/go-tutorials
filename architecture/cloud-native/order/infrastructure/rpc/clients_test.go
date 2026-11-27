// RPC 客户端单元测试 —— 三个教学点（超时 / 重试 / 幂等前提）在这里逐条验证。
//
// 对应 Java 版没有专门单测的 RpcClient 逻辑，Go 版把它变成可执行的规约：
//   - 网络层错误（连接中断）→ 重试，最多 2 次；
//   - 业务错误（HTTP 5xx/4xx）→ 绝不重试；
//   - 对端过慢 → 在读超时处失败，而不是无限等待。
package rpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// TestPayRetriesNetworkErrorsUntilSuccess 验证：
// 第一次请求连接被对端中断（网络层错误，请求结果未知）→ 自动重试 → 第二次成功。
// 这正是 payment 按 orderId 幂等存在的意义：POST 重放不会产生第二笔支付。
func TestPayRetriesNetworkErrorsUntilSuccess(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rpc/payments/pay" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if q := r.URL.Query(); q.Get("orderId") != "order-1" || q.Get("amount") != "19.99" {
			t.Errorf("unexpected query %v", q)
		}
		n := atomic.AddInt32(&hits, 1)
		if n == 1 {
			// 模拟网络层错误：连接被粗暴中断，服务端是否处理了请求并不确定。
			panic(http.ErrAbortHandler)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("payment-1"))
	}))
	defer func() { srv.Close() }()

	c := NewPaymentClient(srv.URL)
	got, err := c.Pay(context.Background(), "order-1", decimal.RequireFromString("19.99"))
	if err != nil {
		t.Fatalf("pay after retry: %v", err)
	}
	if got != "payment-1" {
		t.Fatalf("paymentId = %q, want payment-1", got)
	}
	if n := atomic.LoadInt32(&hits); n != 2 {
		t.Fatalf("requests = %d, want 2 (1 network failure + 1 retry)", n)
	}
}

// TestPayDoesNotRetryBusinessErrors 验证：拿到 HTTP 状态码的业务错误绝不重试
// （对应 Java 只捕获 ResourceAccessException —— 500/404 会被原样抛给调用方）。
func TestPayDoesNotRetryBusinessErrors(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"errorCode":"PAYMENT_FAILED","message":"insufficient funds"}`))
	}))
	defer func() { srv.Close() }()

	c := NewPaymentClient(srv.URL)
	_, err := c.Pay(context.Background(), "order-1", decimal.RequireFromString("19.99"))
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("want HTTP 500 business error, got %v", err)
	}
	if n := atomic.LoadInt32(&hits); n != 1 {
		t.Fatalf("requests = %d, want 1 (business errors are never retried)", n)
	}
}

// TestPayTimesOutWhenPeerTooSlow 验证读超时：对端挂起时不无限等待
// （对应 Java readTimeout=3000ms；测试用 50ms 的短超时版本验证机制本身）。
func TestPayTimesOutWhenPeerTooSlow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer func() { srv.Close() }()

	// 白盒构造：读超时压到 50ms（生产值是常量 readTimeout = 3s）。
	slow := &http.Client{Transport: &http.Transport{ResponseHeaderTimeout: 50 * time.Millisecond}}
	c := &PaymentClient{baseURL: srv.URL, client: slow}

	start := time.Now()
	_, err := c.Pay(context.Background(), "order-slow", decimal.RequireFromString("1.00"))
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "attempt 2/2") {
		t.Fatalf("want retry-exhausted error, got %v", err)
	}
	if elapsed >= 300*time.Millisecond {
		t.Fatalf("elapsed = %s, timeout should have cut the wait far below the peer's 300ms stall", elapsed)
	}
}

// TestRefundMapsToRefundEndpoint 验证 refund 的路径映射与错误透传。
func TestRefundMapsToRefundEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/refund") {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/pay-9/refund"):
			w.WriteHeader(http.StatusOK)
		default:
			// 对应 payment 服务的 PAYMENT_NOT_FOUND → 404
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errorCode":"PAYMENT_NOT_FOUND"}`))
		}
	}))
	defer func() { srv.Close() }()

	c := NewPaymentClient(srv.URL)
	if err := c.Refund(context.Background(), "pay-9"); err != nil {
		t.Fatalf("refund pay-9: %v", err)
	}
	err := c.Refund(context.Background(), "pay-404")
	if err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Fatalf("want HTTP 404 error, got %v", err)
	}
}

// TestProductClientDecodesJSONContract 验证 product 契约的 JSON 往返：
// DTO 的 json tag 就是跨服务的字段名协议（对应 Java record 的 Jackson 序列化）。
func TestProductClientDecodesJSONContract(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/rpc/products/p-1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"productId": "p-1", "name": "Laptop", "price": "999.99", "stock": 10,
			})
		case r.URL.Path == "/rpc/products":
			if got := r.URL.Query().Get("ids"); got != "p-1,p-2" {
				t.Errorf("ids query = %q, want p-1,p-2", got)
			}
			// 与 Java 一致：不存在的 ID（p-2）不报错，只返回找到的部分
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"productId": "p-1", "name": "Laptop", "price": "999.99", "stock": 10},
			})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer func() { srv.Close() }()

	c := NewProductClient(srv.URL)

	dto, err := c.GetProduct(context.Background(), "p-1")
	if err != nil {
		t.Fatalf("get product: %v", err)
	}
	if dto.Name != "Laptop" || dto.Stock != 10 || dto.Price.StringFixed(2) != "999.99" {
		t.Fatalf("decoded product = %+v", dto)
	}

	list, err := c.GetProducts(context.Background(), []string{"p-1", "p-2"})
	if err != nil {
		t.Fatalf("get products: %v", err)
	}
	if len(list) != 1 || list[0].ProductID != "p-1" {
		t.Fatalf("decoded products = %+v, want only p-1", list)
	}
}
