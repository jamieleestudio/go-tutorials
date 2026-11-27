// Package rpc 订单基础设施的跨服务 HTTP RPC 客户端。
//
// 对应 Java 的 infrastructure/rpc 包（PaymentServiceRpcClient / ProductServiceRpcClient）：
// 把 api 包里的 provider 契约适配到 HTTP 上。没有注册中心、没有 gRPC —— P7 的
// 服务发现就是 K8s Service DNS（固定域名），客户端保持"朴素"照旧成立，
// 把分布式调用最要命的三件事摊开在同一处：
//
//   - 超时（timeout）：连接 2s / 等响应 3s，对应 Java RestClient 的
//     connectTimeout=2000ms / readTimeout=3000ms。不设超时的 RPC 是事故之源。
//   - 重试（retry）：最多尝试 maxAttempts=2 次，且只重试"网络层错误"
//     （client.Do 返回 error，对应 Java 的 ResourceAccessException）——
//     请求可能根本没到达对端。拿到 HTTP 状态码的业务错误一律不重试。
//   - 幂等（idempotency）：网络层重试意味着同一个 POST 可能被执行两次。
//     POST 天然不幂等，客户端敢重试的前提是提供方幂等 —— payment 按
//     orderId 幂等（同单同 ID），这正是这组设计的闭环所在。
package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	paymentapi "github.com/jamieleestudio/go-tutorials/architecture/cloud-native/payment/api"
	productapi "github.com/jamieleestudio/go-tutorials/architecture/cloud-native/product/api"

	"github.com/shopspring/decimal"
)

const (
	// connectTimeout TCP 连接超时（对应 Java connectTimeout=2000ms）。
	connectTimeout = 2 * time.Second
	// readTimeout 等待对端响应头的超时（对应 Java readTimeout=3000ms）。
	readTimeout = 3 * time.Second
	// maxAttempts 单次业务调用最多尝试的次数（对应 Java MAX_ATTEMPTS=2）。
	maxAttempts = 2
)

// newHTTPClient 构造带超时的 HTTP 客户端（对应 Java 里 RestClient.builder() 的超时配置）。
func newHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			// 连接超时 2s。
			DialContext: (&net.Dialer{Timeout: connectTimeout}).DialContext,
			// 读超时 3s（等待对端响应头）。
			ResponseHeaderTimeout: readTimeout,
		},
	}
}

// doWithRetry 发起请求，网络层错误最多尝试 maxAttempts 次。
func doWithRetry(ctx context.Context, client *http.Client, method, rawURL string) (*http.Response, error) {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
		if err != nil {
			return nil, fmt.Errorf("rpc: build request %s %s: %w", method, rawURL, err)
		}
		resp, err := client.Do(req)
		if err != nil {
			// 网络层错误：请求可能没到达对端，也可能响应在途中丢失。
			// 只有提供方幂等（payment Pay 按 orderId 幂等），这里才敢再发一次。
			lastErr = fmt.Errorf("rpc: %s %s attempt %d/%d: %w", method, rawURL, attempt, maxAttempts, err)
			if attempt < maxAttempts {
				slog.Warn("rpc network error, will retry",
					"method", method, "url", rawURL, "attempt", attempt, "error", err)
			}
			continue
		}
		return resp, nil
	}
	return nil, lastErr
}

// checkStatus 非 2xx 一律视为业务错误 —— 拿到了响应就不算网络层错误，绝不重试。
func checkStatus(resp *http.Response, what string) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	return fmt.Errorf("rpc: %s: HTTP %d: %s", what, resp.StatusCode, strings.TrimSpace(string(body)))
}

// PaymentClient api.PaymentService（payment provider 契约）的 HTTP RPC 实现。
// 对应 Java 的 PaymentServiceRpcClient。
type PaymentClient struct {
	baseURL string
	client  *http.Client
}

// NewPaymentClient 构造 payment RPC 客户端（baseURL 形如 http://127.0.0.1:8082）。
func NewPaymentClient(baseURL string) *PaymentClient {
	return &PaymentClient{baseURL: strings.TrimRight(baseURL, "/"), client: newHTTPClient()}
}

// Pay 幂等支付：POST /rpc/payments/pay?orderId=&amount=。
// amount 用 StringFixed(2) 走 query 参数（对应 Java 的 BigDecimal.toString），
// 成功时响应体是纯文本 paymentId（与 Java 版一致，不是 JSON）。
func (c *PaymentClient) Pay(ctx context.Context, orderID string, amount decimal.Decimal) (string, error) {
	u := fmt.Sprintf("%s/rpc/payments/pay?orderId=%s&amount=%s",
		c.baseURL, url.QueryEscape(orderID), url.QueryEscape(amount.StringFixed(2)))

	resp, err := doWithRetry(ctx, c.client, http.MethodPost, u)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if err := checkStatus(resp, "pay "+orderID); err != nil {
		return "", err
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", fmt.Errorf("rpc: read pay response: %w", err)
	}
	return strings.TrimSpace(string(body)), nil
}

// Refund 退款：POST /rpc/payments/{paymentId}/refund（200 空响应体）。
func (c *PaymentClient) Refund(ctx context.Context, paymentID string) error {
	u := fmt.Sprintf("%s/rpc/payments/%s/refund", c.baseURL, url.PathEscape(paymentID))

	resp, err := doWithRetry(ctx, c.client, http.MethodPost, u)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	return checkStatus(resp, "refund "+paymentID)
}

// ProductClient api.ProductService（product provider 契约）的 HTTP RPC 实现。
// 对应 Java 的 ProductServiceRpcClient。
type ProductClient struct {
	baseURL string
	client  *http.Client
}

// NewProductClient 构造 product RPC 客户端。
func NewProductClient(baseURL string) *ProductClient {
	return &ProductClient{baseURL: strings.TrimRight(baseURL, "/"), client: newHTTPClient()}
}

// GetProduct GET /rpc/products/{productId} → ProductDto JSON。
func (c *ProductClient) GetProduct(ctx context.Context, productID string) (*productapi.ProductDto, error) {
	u := fmt.Sprintf("%s/rpc/products/%s", c.baseURL, url.PathEscape(productID))

	resp, err := doWithRetry(ctx, c.client, http.MethodGet, u)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if err := checkStatus(resp, "get product "+productID); err != nil {
		return nil, err
	}
	var dto productapi.ProductDto
	if err := json.NewDecoder(resp.Body).Decode(&dto); err != nil {
		return nil, fmt.Errorf("rpc: decode product %s: %w", productID, err)
	}
	return &dto, nil
}

// GetProducts GET /rpc/products?ids=a,b → []ProductDto JSON。
// 与 Java 一致：不存在的 ID 不报错，只返回找到的部分 —— 由 order 侧自行判断缺品。
func (c *ProductClient) GetProducts(ctx context.Context, productIDs []string) ([]*productapi.ProductDto, error) {
	u := fmt.Sprintf("%s/rpc/products?ids=%s", c.baseURL, url.QueryEscape(strings.Join(productIDs, ",")))

	resp, err := doWithRetry(ctx, c.client, http.MethodGet, u)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if err := checkStatus(resp, "get products"); err != nil {
		return nil, err
	}
	dtos := make([]*productapi.ProductDto, 0)
	if err := json.NewDecoder(resp.Body).Decode(&dtos); err != nil {
		return nil, fmt.Errorf("rpc: decode products: %w", err)
	}
	return dtos, nil
}

// 编译期确认：两个客户端都是 provider 契约的合法实现。
var (
	_ paymentapi.PaymentService = (*PaymentClient)(nil)
	_ productapi.ProductService = (*ProductClient)(nil)
)
