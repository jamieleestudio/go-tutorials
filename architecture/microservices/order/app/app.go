// Package app order 服务的组合根（对应 Java 的 @SpringBootApplication）。
//
// 与 product/payment 不同：order 没有自己的 gRPC server —— 它是 RPC 消费方，
// 在这里把 product / payment 的 gRPC 地址装配成契约客户端（对应 Java 版
// Feign 客户端 + Nacos 地址解析；P4 用 env 固定地址，见 README 演进说明）。
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

	"github.com/jamieleestudio/go-tutorials/architecture/microservices/order/application/impl"
	orderinf "github.com/jamieleestudio/go-tutorials/architecture/microservices/order/infrastructure"
	orderinfevent "github.com/jamieleestudio/go-tutorials/architecture/microservices/order/infrastructure/event"
	ordergrpc "github.com/jamieleestudio/go-tutorials/architecture/microservices/order/infrastructure/grpc"
	"github.com/jamieleestudio/go-tutorials/architecture/microservices/order/interfaces/httpmw"
	orderweb "github.com/jamieleestudio/go-tutorials/architecture/microservices/order/interfaces/web"
	"github.com/jamieleestudio/go-tutorials/architecture/microservices/shared"
	"github.com/jamieleestudio/go-tutorials/architecture/microservices/shared/persistence"
)

// Config 服务装配配置（对应 Java 的 application.yml）。
type Config struct {
	HTTPAddr        string // REST 监听地址，默认 :8081
	ProductGRPCAddr string // product 的 gRPC 地址，默认 :9183
	PaymentGRPCAddr string // payment 的 gRPC 地址，默认 :9182
	DSN             string // 数据库 DSN，默认 SQLite 内存库
	LogLevel        string
}

// DefaultConfig 从环境变量读配置。
// PRODUCT_GRPC_ADDR / PAYMENT_GRPC_ADDR 就是 P4 的"服务发现"——
// Nacos 对应物的演进接缝。
func DefaultConfig() Config {
	return Config{
		HTTPAddr:        envOr("ORDER_HTTP_ADDR", ":8081"),
		ProductGRPCAddr: envOr("PRODUCT_GRPC_ADDR", ":9183"),
		PaymentGRPCAddr: envOr("PAYMENT_GRPC_ADDR", ":9182"),
		DSN:             envOr("DB_DSN", "sqlite://file:order?mode=memory&cache=shared"),
		LogLevel:        envOr("DB_LOG_LEVEL", "warn"),
	}
}

// NewHTTPServer 装配 REST 路由。
func NewHTTPServer(appService *impl.OrderServiceImpl) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), httpmw.ErrorHandler())
	orderweb.NewOrderController(appService).Register(r)
	return r
}

// NewRepositories 打开订单库 + 建表 + 构造仓储（同时返回句柄给事务运行器）。
func NewRepositories(cfg Config) (*orderinf.OrderRepositoryImpl, *gorm.DB, error) {
	db, err := persistence.Open(cfg.DSN, cfg.LogLevel)
	if err != nil {
		return nil, nil, err
	}
	if err := persistence.Migrate(db, &orderinf.OrderEntity{}, &orderinf.OrderItemEntity{}); err != nil {
		return nil, nil, fmt.Errorf("migrate order db: %w", err)
	}
	return orderinf.NewOrderRepository(db), db, nil
}

// Run 装配并启动 order 服务，阻塞直到 ctx 取消。
// 依赖链：db → 远程契约客户端（gRPC + 熔断）→ 应用服务 → REST。
func Run(ctx context.Context, cfg Config) error {
	repo, db, err := NewRepositories(cfg)
	if err != nil {
		return err
	}

	// 远程契约客户端：Feign 对应物（gRPC + gobreaker + deadline）
	productClient, err := ordergrpc.NewProductClient(cfg.ProductGRPCAddr)
	if err != nil {
		return err
	}
	defer func() { _ = productClient.Close() }()

	paymentClient, err := ordergrpc.NewPaymentClient(cfg.PaymentGRPCAddr)
	if err != nil {
		return err
	}
	defer func() { _ = paymentClient.Close() }()

	// 事件装配：进程内总线 + 发布者（application 端口的实现）+ 监听器（interfaces 职责）
	eventBus := shared.NewEventBus()
	publisher := orderinfevent.NewEventPublisher(eventBus)
	orderinfevent.NewOrderEventListener().Register(eventBus)

	appService := impl.NewOrderServiceImpl(
		repo,
		persistence.NewGormTxRunner(db),
		productClient,
		paymentClient,
		publisher,
	)

	server := &http.Server{Addr: cfg.HTTPAddr, Handler: NewHTTPServer(appService), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		slog.Info("order HTTP listening", "addr", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("order http server error", "error", err)
		}
	}()

	<-ctx.Done()
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
