// Package gateway API 网关 —— 统一入口 8080（对应 Java 的 ms-api-gateway / Spring Cloud Gateway）。
//
// 路由表与 Java 版 application.yml 一致：
//
//	/api/orders/**   → order-service   (lb://order-service)
//	/api/payments/** → payment-service (lb://payment-service)
//	/api/products/** → product-service (lb://product-service)
//
// Java 版经 Nacos 服务发现 + LoadBalancer 解析 lb://；Go 版直接用可配置的
// 上游地址（env），服务发现的演进接缝见 README。
package gateway

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/gin-gonic/gin"
)

// Routes 上游路由表。
type Routes struct {
	OrderURL   string
	PaymentURL string
	ProductURL string
}

// DefaultRoutes 从环境变量读上游地址。
// ORDER_SERVICE_URL / PAYMENT_SERVICE_URL / PRODUCT_SERVICE_URL 就是 P4 的
// "服务发现"（Nacos 对应物），见 README 的演进说明。
func DefaultRoutes() Routes {
	return Routes{
		OrderURL:   envOr("ORDER_SERVICE_URL", "http://localhost:8081"),
		PaymentURL: envOr("PAYMENT_SERVICE_URL", "http://localhost:8082"),
		ProductURL: envOr("PRODUCT_SERVICE_URL", "http://localhost:8083"),
	}
}

// New 装配网关路由。
func New(routes Routes) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	register(r, "/api/orders", routes.OrderURL)
	register(r, "/api/payments", routes.PaymentURL)
	register(r, "/api/products", routes.ProductURL)
	return r
}

func register(r *gin.Engine, prefix, target string) {
	u, err := url.Parse(target)
	if err != nil {
		panic(err) // 配置错误在启动期就该暴露
	}
	proxy := httputil.NewSingleHostReverseProxy(u)
	// 上游不可达时的降级响应（网关自身的错误契约）
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"errorCode":"SERVICE_UNAVAILABLE","message":"upstream unreachable: ` + target + `"}`))
	}

	handler := func(c *gin.Context) {
		proxy.ServeHTTP(c.Writer, c.Request)
		c.Abort() // 响应已由 proxy 写出，终止后续 handler
	}
	group := r.Group(prefix)
	group.Any("", handler)       // 前缀本身（如 GET /api/products）
	group.Any("/*path", handler) // 前缀下的任意路径
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
