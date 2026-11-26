// Package app payment 服务的组合根（对应 Java 的 @SpringBootApplication）。
//
// P5 的 payment 是纯事件进程：没有 REST 业务端点、没有 gRPC ——
// 唯一的 HTTP 端点是 /internal/events（接收 order 服务的 webhook 投递），
// 加上 Outbox Relay（把 payment-events 投给 order）。
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

	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/payment/application"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/payment/application/impl"
	paymentinf "github.com/jamieleestudio/go-tutorials/architecture/event-driven/payment/infrastructure"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/payment/infrastructure/outbox"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/payment/infrastructure/processed"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/payment/interfaces/httpmw"
	paymentmessaging "github.com/jamieleestudio/go-tutorials/architecture/event-driven/payment/interfaces/messaging"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared/event"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared/persistence"
)

// Config 服务装配配置（对应 Java 的 application.yml）。
type Config struct {
	HTTPAddr string // HTTP 监听地址（仅事件入口），默认 :8082
	DSN      string // 数据库 DSN，默认 SQLite 内存库
	LogLevel string // gorm 日志级别

	// RelayInterval Outbox 轮询间隔（对应 Java @Scheduled fixedDelay=1000）。
	RelayInterval time.Duration
	// EventSubscribers topic → 订阅者端点列表（webhook 投递目标）。
	EventSubscribers map[string][]string
}

// DefaultConfig 从环境变量读配置，未设置时取与 Java 版一致的默认值。
func DefaultConfig() Config {
	return Config{
		HTTPAddr:      envOr("PAYMENT_HTTP_ADDR", ":8082"),
		DSN:           envOr("DB_DSN", "sqlite://file:eda-payment?mode=memory&cache=shared"),
		LogLevel:      envOr("DB_LOG_LEVEL", "warn"),
		RelayInterval: envDurationOr("OUTBOX_RELAY_INTERVAL", time.Second),
		// 本服务发布 payment-events → 订阅者是 order 服务
		EventSubscribers: map[string][]string{
			event.TopicPaymentEvents: envListOr("PAYMENT_EVENTS_SUBSCRIBERS",
				[]string{"http://localhost:8081/internal/events"}),
		},
	}
}

// Repositories 服务全部仓储与事务设施（一个服务独享一个数据库）。
type Repositories struct {
	DB             *gorm.DB
	TxRunner       shared.TxRunner
	Payments       *paymentinf.PaymentRepositoryImpl
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
		&paymentinf.PaymentEntity{},
		&outbox.OutboxMessageEntity{},
		&processed.ProcessedEventEntity{},
	); err != nil {
		return nil, fmt.Errorf("migrate payment db: %w", err)
	}
	outboxRepo := outbox.NewOutboxRepository(db)
	return &Repositories{
		DB:             db,
		TxRunner:       persistence.NewGormTxRunner(db),
		Payments:       paymentinf.NewPaymentRepository(db),
		Outbox:         outboxRepo,
		EventPublisher: outbox.NewOutboxEventPublisher(outboxRepo),
		Dedup:          processed.NewDeduplicationService(db),
	}, nil
}

// NewAppService 装配应用服务（依赖注入全部在此显式完成）。
func NewAppService(r *Repositories) application.PaymentSagaService {
	return impl.NewPaymentSagaServiceImpl(r.Payments, r.TxRunner, r.EventPublisher, r.Dedup)
}

// NewHTTPServer 装配 HTTP 路由 —— 只有事件入口（payment 是纯事件进程）。
func NewHTTPServer(svc application.PaymentSagaService) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), httpmw.ErrorHandler())
	paymentmessaging.NewOrderEventConsumer(svc).Register(r)
	return r
}

// NewRelay 装配 Outbox Relay（业务事务内落库 → 异步 webhook 投递）。
func NewRelay(r *Repositories, cfg Config) *outbox.Relay {
	return outbox.NewRelay(r.Outbox, outbox.HTTPDeliver(cfg.EventSubscribers), cfg.RelayInterval)
}

// Run 启动 Relay（后台）+ HTTP，阻塞直到 ctx 取消。
func Run(ctx context.Context, cfg Config) error {
	repos, err := NewRepositories(cfg)
	if err != nil {
		return err
	}
	svc := NewAppService(repos)

	go NewRelay(repos, cfg).Run(ctx)

	httpErr := make(chan error, 1)
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: NewHTTPServer(svc), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		slog.Info("payment HTTP listening", "addr", cfg.HTTPAddr)
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
