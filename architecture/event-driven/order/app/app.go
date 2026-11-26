// Package app order 服务的组合根（对应 Java 的 @SpringBootApplication）。
//
// 单独成包而非塞在 cmd/main.go 里，有两个原因：
//  1. e2e 测试可以在进程内用随机端口装配整个服务（对应 @SpringBootTest）；
//  2. cmd/main.go 保持"只读配置 + 调用 Run"的最小形态。
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/order/application"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/order/application/impl"
	orderinf "github.com/jamieleestudio/go-tutorials/architecture/event-driven/order/infrastructure"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/order/infrastructure/grpc"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/order/infrastructure/outbox"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/order/infrastructure/processed"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/order/interfaces/httpmw"
	ordermessaging "github.com/jamieleestudio/go-tutorials/architecture/event-driven/order/interfaces/messaging"
	orderweb "github.com/jamieleestudio/go-tutorials/architecture/event-driven/order/interfaces/web"
	productapi "github.com/jamieleestudio/go-tutorials/architecture/event-driven/product/api"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared/event"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared/persistence"
)

// Config 服务装配配置（对应 Java 的 application.yml）。
type Config struct {
	HTTPAddr string // REST 监听地址，默认 :8081
	DSN      string // 数据库 DSN，默认 SQLite 内存库
	LogLevel string // gorm 日志级别

	// ProductGRPCAddr product 服务的 gRPC 地址（同步目录读）。
	ProductGRPCAddr string
	// RelayInterval Outbox 轮询间隔（对应 Java @Scheduled fixedDelay=1000）。
	RelayInterval time.Duration
	// EventSubscribers topic → 订阅者端点列表（webhook 投递目标）。
	EventSubscribers map[string][]string
}

// DefaultConfig 从环境变量读配置，未设置时取与 Java 版一致的默认值。
func DefaultConfig() Config {
	return Config{
		HTTPAddr:        envOr("ORDER_HTTP_ADDR", ":8081"),
		DSN:             envOr("DB_DSN", "sqlite://file:eda-order?mode=memory&cache=shared"),
		LogLevel:        envOr("DB_LOG_LEVEL", "warn"),
		ProductGRPCAddr: envOr("PRODUCT_GRPC_ADDR", "localhost:9183"),
		RelayInterval:   envDurationOr("OUTBOX_RELAY_INTERVAL", time.Second),
		// 本服务发布 order-events → 订阅者是 payment + product 两个服务
		EventSubscribers: map[string][]string{
			event.TopicOrderEvents: envListOr("ORDER_EVENTS_SUBSCRIBERS",
				[]string{
					"http://localhost:8082/internal/events",
					"http://localhost:8083/internal/events",
				}),
		},
	}
}

// Repositories 服务全部仓储与事务设施（一个服务独享一个数据库）。
type Repositories struct {
	DB             *gorm.DB
	TxRunner       shared.TxRunner
	Orders         *orderinf.OrderRepositoryImpl
	Outbox         outbox.OutboxRepository
	EventPublisher *outbox.OutboxEventPublisher
	Dedup          *processed.DeduplicationService
}

// NewRepositories 打开数据库 + 建表 + 构造全部仓储。
func NewRepositories(cfg Config) (*Repositories, error) {
	db, err := persistence.Open(cfg.DSN, cfg.LogLevel)
	if err != nil {
		return nil, err
	}
	if err := persistence.Migrate(db,
		&orderinf.OrderEntity{},
		&orderinf.OrderItemEntity{},
		&outbox.OutboxMessageEntity{},
		&processed.ProcessedEventEntity{},
	); err != nil {
		return nil, fmt.Errorf("migrate order db: %w", err)
	}
	outboxRepo := outbox.NewOutboxRepository(db)
	return &Repositories{
		DB:             db,
		TxRunner:       persistence.NewGormTxRunner(db),
		Orders:         orderinf.NewOrderRepository(db),
		Outbox:         outboxRepo,
		EventPublisher: outbox.NewOutboxEventPublisher(outboxRepo),
		Dedup:          processed.NewDeduplicationService(db),
	}, nil
}

// NewAppService 装配应用服务（依赖注入全部在此显式完成）。
// productClient 是基础设施层的 gRPC 适配器 —— application 只认识契约接口。
func NewAppService(r *Repositories, productClient productapi.ProductService) application.OrderService {
	return impl.NewOrderServiceImpl(r.Orders, productClient, r.TxRunner, r.EventPublisher, r.Dedup)
}

// NewHTTPServer 装配 REST 路由（业务端点 + Saga 事件入口）。
func NewHTTPServer(svc application.OrderService) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), httpmw.ErrorHandler())
	orderweb.NewOrderController(svc).Register(r)
	ordermessaging.NewSagaEventConsumer(svc).Register(r)
	return r
}

// NewRelay 装配 Outbox Relay（业务事务内落库 → 异步 webhook 投递）。
func NewRelay(r *Repositories, cfg Config) *outbox.Relay {
	return outbox.NewRelay(r.Outbox, outbox.HTTPDeliver(cfg.EventSubscribers), cfg.RelayInterval)
}

// NewProductClient 装配 product 契约的 gRPC 客户端。
func NewProductClient(addr string) (*grpc.ProductClient, error) {
	return grpc.NewProductClient(addr)
}

// Run 启动 Relay（后台）+ HTTP，阻塞直到 ctx 取消。
func Run(ctx context.Context, cfg Config) error {
	repos, err := NewRepositories(cfg)
	if err != nil {
		return err
	}
	productClient, err := NewProductClient(cfg.ProductGRPCAddr)
	if err != nil {
		return fmt.Errorf("create product client: %w", err)
	}
	defer func() { _ = productClient.Close() }()

	svc := NewAppService(repos, productClient)
	go NewRelay(repos, cfg).Run(ctx)

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

func envDurationOr(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

// envListOr 逗号分隔的端点列表（订阅者配置）。
func envListOr(key string, fallback []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	var items []string
	for _, part := range strings.Split(v, ",") {
		if s := strings.TrimSpace(part); s != "" {
			items = append(items, s)
		}
	}
	if len(items) == 0 {
		return fallback
	}
	return items
}
