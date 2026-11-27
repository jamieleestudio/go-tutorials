// Package app order 服务的组合根（对应 Java cn-order-service 的 @SpringBootApplication）。
//
// order 是三个服务里唯一的"编排者"：REST /api/orders 接客；跨服务调用走
// infrastructure/rpc 的 HTTP 客户端 —— P7 里基址默认是 K8s Service DNS
// （http://payment-service:8080），【服务发现就是 DNS】，没有注册中心；
// 领域事件走进程内总线（对应 Spring @EventListener）。
//
// P7 与 P6 的分工刻意保持"业务装配零修改"（对应 Java 版 ③→⑥ 业务代码逐字节
// 相同）：变化只在运行时外壳 —— 统一 :8080、健康探针/指标、优雅停机（cloudkit）。
// 装配逻辑不放在 cmd/main.go：e2e 测试仍可在进程内按随机端口装配业务路由。
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/jamieleestudio/go-tutorials/architecture/cloud-native/order/application"
	"github.com/jamieleestudio/go-tutorials/architecture/cloud-native/order/application/impl"
	orderinf "github.com/jamieleestudio/go-tutorials/architecture/cloud-native/order/infrastructure"
	orderinfevent "github.com/jamieleestudio/go-tutorials/architecture/cloud-native/order/infrastructure/event"
	orderrpc "github.com/jamieleestudio/go-tutorials/architecture/cloud-native/order/infrastructure/rpc"
	"github.com/jamieleestudio/go-tutorials/architecture/cloud-native/order/interfaces/httpmw"
	orderweb "github.com/jamieleestudio/go-tutorials/architecture/cloud-native/order/interfaces/web"
	paymentapi "github.com/jamieleestudio/go-tutorials/architecture/cloud-native/payment/api"
	productapi "github.com/jamieleestudio/go-tutorials/architecture/cloud-native/product/api"
	"github.com/jamieleestudio/go-tutorials/architecture/cloud-native/shared"
	"github.com/jamieleestudio/go-tutorials/architecture/cloud-native/shared/cloudkit"
	"github.com/jamieleestudio/go-tutorials/architecture/cloud-native/shared/persistence"
)

// Config 装配配置（对应 Java 的 application.yml —— 12-Factor：全部 ${ENV:default}）。
type Config struct {
	HTTPAddr string // REST 端口；K8s 惯例统一 :8080（PORT 环境变量可覆盖）
	DSN      string
	LogLevel string

	// PaymentServiceURL payment 服务的基址 —— 默认是 K8s Service DNS（对应 Java
	// application.yml 的 payment.service.base-url: ${PAYMENT_SERVICE_URL:http://payment-service:8080}）。
	// 服务发现的全部实现就是【平台提供的 DNS 名】。
	PaymentServiceURL string
	// ProductServiceURL product 服务的基址（PRODUCT_SERVICE_URL 可覆盖）。
	ProductServiceURL string
}

// DefaultConfig 从环境变量读配置：URL 默认值与 Java 版逐字一致（K8s Service DNS）。
func DefaultConfig() Config {
	return Config{
		HTTPAddr:          cloudkit.AddrFromPort("8080"),
		DSN:               cloudkit.EnvOr("DB_DSN", "sqlite://file:cn-order?mode=memory&cache=shared"),
		LogLevel:          cloudkit.EnvOr("DB_LOG_LEVEL", "warn"),
		PaymentServiceURL: cloudkit.EnvOr("PAYMENT_SERVICE_URL", "http://payment-service:8080"),
		ProductServiceURL: cloudkit.EnvOr("PRODUCT_SERVICE_URL", "http://product-service:8080"),
	}
}

// Repositories 全局持久化设施（一个服务一个数据库）。
type Repositories struct {
	DB     *gorm.DB
	Orders *orderinf.OrderRepositoryImpl
}

// NewRepositories 打开数据库 + 迁移 + 构造仓储。
func NewRepositories(cfg Config) (*Repositories, error) {
	db, err := persistence.Open(cfg.DSN, cfg.LogLevel)
	if err != nil {
		return nil, err
	}
	if err := persistence.Migrate(db, &orderinf.OrderEntity{}, &orderinf.OrderItemEntity{}); err != nil {
		return nil, fmt.Errorf("migrate order db: %w", err)
	}
	return &Repositories{DB: db, Orders: orderinf.NewOrderRepository(db)}, nil
}

// NewPublisher 进程内事件装配：总线 + 发布器 + 监听器。
// 对应 Spring 的 ApplicationEventPublisher + @EventListener —— 全部同进程。
func NewPublisher() (application.EventPublisherService, *shared.EventBus) {
	bus := shared.NewEventBus()
	publisher := orderinfevent.NewEventPublisher(bus)
	orderinfevent.NewOrderEventListener().Register(bus)
	return publisher, bus
}

// NewRPCClients 装配两个远程契约的 HTTP 客户端（对应 Java 的 RPC client bean；
// 基址 = K8s Service DNS，e2e 里被真实随机端口覆盖 —— env 覆盖同一机制）。
func NewRPCClients(cfg Config) (*orderrpc.PaymentClient, *orderrpc.ProductClient) {
	return orderrpc.NewPaymentClient(cfg.PaymentServiceURL), orderrpc.NewProductClient(cfg.ProductServiceURL)
}

// NewAppService 装配应用服务：本地仓储 + 两个远程契约（provider-defined）+ 事件端口。
func NewAppService(
	r *Repositories,
	payments paymentapi.PaymentService,
	products productapi.ProductService,
	publisher application.EventPublisherService,
) application.OrderService {
	return impl.NewOrderServiceImpl(r.Orders, persistence.NewGormTxRunner(r.DB), products, payments, publisher)
}

// NewHTTPServer 装配 REST 路由（order 只有对外端点；跨服务调用全在客户端侧）。
// 只含业务端点 —— 云原生运行时端点（探针/指标）由 Run 挂载。
func NewHTTPServer(svc application.OrderService) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), httpmw.ErrorHandler())
	orderweb.NewOrderController(svc).Register(r)
	return r
}

// Run 装配并运行 order 服务，直到 ctx 取消（SIGTERM/SIGINT → 优雅停机）。
func Run(ctx context.Context, cfg Config) error {
	repos, err := NewRepositories(cfg)
	if err != nil {
		return err
	}
	publisher, _ := NewPublisher()
	payments, products := NewRPCClients(cfg)
	svc := NewAppService(repos, payments, products, publisher)

	health := cloudkit.NewHealth("order")
	metrics := cloudkit.NewMetrics(health)

	engine := NewHTTPServer(svc)
	for _, rt := range cloudkit.Routes(health, metrics) {
		engine.Handle(rt.Method, rt.Path, gin.WrapF(rt.Handler)) // /healthz/liveness /healthz/readiness /healthz /info /metrics
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           metrics.Middleware(engine),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// bind 先于 ready 翻转：端口开着但 readiness 503 的窗口里，K8s 不放流量进来。
	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.HTTPAddr, err)
	}

	health.SetReady(true) // 装配（含 DB 迁移）完成 → 放行 readiness
	slog.Info("order HTTP listening", "addr", ln.Addr().String(),
		"probes", "/healthz/liveness /healthz/readiness",
		"payment", cfg.PaymentServiceURL, "product", cfg.ProductServiceURL)

	// 优雅停机：SIGTERM → readiness=false（摘流）→ 排水存量请求（8s 窗口）→ 关库。
	return cloudkit.Serve(ctx, health, server, ln, 8*time.Second, func() error {
		sqlDB, err := repos.DB.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	})
}
