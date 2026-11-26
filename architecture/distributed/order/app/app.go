// Package app order 服务的组合根（对应 Java dist-order-service 的 @SpringBootApplication）。
//
// order 是三个服务里唯一的"编排者"：REST /api/orders 接客；跨服务调用走
// infrastructure/rpc 的 HTTP 客户端（对应 Java 的两个 RPC client bean，无注册
// 中心、env 固定地址）；领域事件走进程内总线（对应 Spring @EventListener）。
// 装配逻辑不放在 cmd/main.go —— e2e 测试要在进程内按随机端口装配整个服务。
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/jamieleestudio/go-tutorials/architecture/distributed/order/application"
	"github.com/jamieleestudio/go-tutorials/architecture/distributed/order/application/impl"
	orderinf "github.com/jamieleestudio/go-tutorials/architecture/distributed/order/infrastructure"
	orderinfevent "github.com/jamieleestudio/go-tutorials/architecture/distributed/order/infrastructure/event"
	orderrpc "github.com/jamieleestudio/go-tutorials/architecture/distributed/order/infrastructure/rpc"
	"github.com/jamieleestudio/go-tutorials/architecture/distributed/order/interfaces/httpmw"
	orderweb "github.com/jamieleestudio/go-tutorials/architecture/distributed/order/interfaces/web"
	paymentapi "github.com/jamieleestudio/go-tutorials/architecture/distributed/payment/api"
	productapi "github.com/jamieleestudio/go-tutorials/architecture/distributed/product/api"
	"github.com/jamieleestudio/go-tutorials/architecture/distributed/shared"
	"github.com/jamieleestudio/go-tutorials/architecture/distributed/shared/persistence"
)

// Config 装配配置（对应 Java 的 application.yml —— P6 无注册中心，固定 URL）。
type Config struct {
	HTTPAddr string // REST 端口，默认 :8081
	DSN      string
	LogLevel string

	// PaymentServiceURL payment 服务的固定地址（对应 Java application.yml 的
	// payment-service base-url；PAYMENT_SERVICE_URL 可覆盖）。
	PaymentServiceURL string
	// ProductServiceURL product 服务的固定地址（PRODUCT_SERVICE_URL 可覆盖）。
	ProductServiceURL string
}

// DefaultConfig 从环境变量读配置，未设置时取 Java 版同款默认地址。
func DefaultConfig() Config {
	return Config{
		HTTPAddr:          envOr("ORDER_HTTP_ADDR", ":8081"),
		DSN:               envOr("DB_DSN", "sqlite://file:dist-order?mode=memory&cache=shared"),
		LogLevel:          envOr("DB_LOG_LEVEL", "warn"),
		PaymentServiceURL: envOr("PAYMENT_SERVICE_URL", "http://127.0.0.1:8082"),
		ProductServiceURL: envOr("PRODUCT_SERVICE_URL", "http://127.0.0.1:8083"),
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

// NewRPCClients 装配两个远程契约的 HTTP 客户端（对应 Java 的 RPC client bean）。
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
func NewHTTPServer(svc application.OrderService) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), httpmw.ErrorHandler())
	orderweb.NewOrderController(svc).Register(r)
	return r
}

// Run 装配并运行 order 服务，直到 ctx 取消。
func Run(ctx context.Context, cfg Config) error {
	repos, err := NewRepositories(cfg)
	if err != nil {
		return err
	}
	publisher, _ := NewPublisher()
	payments, products := NewRPCClients(cfg)
	svc := NewAppService(repos, payments, products, publisher)

	httpErr := make(chan error, 1)
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: NewHTTPServer(svc), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		slog.Info("order HTTP listening", "addr", cfg.HTTPAddr)
		httpErr <- server.ListenAndServe()
	}()

	select {
	case err := <-httpErr:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
