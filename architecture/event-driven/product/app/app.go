// Package app product 服务的组合根（对应 Java 的 @SpringBootApplication）。
//
// 单独成包而非塞在 cmd/main.go 里，有两个原因：
//  1. e2e 测试可以在进程内用随机端口装配整个服务（对应 @SpringBootTest）；
//  2. cmd/main.go 保持"只读配置 + 调用 Run"的最小形态。
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"gorm.io/gorm"

	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/product/application"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/product/application/impl"
	productinf "github.com/jamieleestudio/go-tutorials/architecture/event-driven/product/infrastructure"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/product/infrastructure/outbox"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/product/infrastructure/processed"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/product/interfaces/httpmw"
	productmessaging "github.com/jamieleestudio/go-tutorials/architecture/event-driven/product/interfaces/messaging"
	productrpc "github.com/jamieleestudio/go-tutorials/architecture/event-driven/product/interfaces/rpc"
	productweb "github.com/jamieleestudio/go-tutorials/architecture/event-driven/product/interfaces/web"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/product/proto/productpb"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared/event"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared/persistence"
)

// Config 服务装配配置（对应 Java 的 application.yml）。
type Config struct {
	HTTPAddr string // REST 监听地址，默认 :8083
	GRPCAddr string // gRPC 监听地址，默认 :9183
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
		HTTPAddr:      envOr("PRODUCT_HTTP_ADDR", ":8083"),
		GRPCAddr:      envOr("PRODUCT_GRPC_ADDR", ":9183"),
		DSN:           envOr("DB_DSN", "sqlite://file:eda-product?mode=memory&cache=shared"),
		LogLevel:      envOr("DB_LOG_LEVEL", "warn"),
		RelayInterval: envDurationOr("OUTBOX_RELAY_INTERVAL", time.Second),
		// 本服务发布 product-events → 订阅者是 order 服务
		EventSubscribers: map[string][]string{
			event.TopicProductEvents: envListOr("PRODUCT_EVENTS_SUBSCRIBERS",
				[]string{"http://localhost:8081/internal/events"}),
		},
	}
}

// Repositories 服务全部仓储与事务设施（一个服务独享一个数据库）。
type Repositories struct {
	DB             *gorm.DB
	TxRunner       shared.TxRunner
	Products       *productinf.ProductRepositoryImpl
	DeductedOrders *productinf.DeductedOrderRepositoryImpl
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
		&productinf.ProductEntity{},
		&productinf.DeductedOrderEntity{},
		&outbox.OutboxMessageEntity{},
		&processed.ProcessedEventEntity{},
	); err != nil {
		return nil, fmt.Errorf("migrate product db: %w", err)
	}
	outboxRepo := outbox.NewOutboxRepository(db)
	return &Repositories{
		DB:             db,
		TxRunner:       persistence.NewGormTxRunner(db),
		Products:       productinf.NewProductRepository(db),
		DeductedOrders: productinf.NewDeductedOrderRepository(db),
		Outbox:         outboxRepo,
		EventPublisher: outbox.NewOutboxEventPublisher(outboxRepo),
		Dedup:          processed.NewDeduplicationService(db),
	}, nil
}

// NewAppService 装配应用服务（依赖注入全部在此显式完成）。
func NewAppService(r *Repositories) application.ProductAppService {
	return impl.NewProductServiceImpl(r.Products, r.DeductedOrders, r.TxRunner, r.EventPublisher, r.Dedup)
}

// NewHTTPServer 装配 REST 路由（业务端点 + 事件入口）。
func NewHTTPServer(svc application.ProductAppService) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), httpmw.ErrorHandler())
	productweb.NewProductController(svc).Register(r)
	productmessaging.NewOrderEventConsumer(svc).Register(r)
	return r
}

// NewGRPCServer 装配 gRPC server（pb.ProductQueryService → api.ProductService）。
func NewGRPCServer(svc application.ProductAppService) *grpc.Server {
	srv := grpc.NewServer()
	productpb.RegisterProductQueryServiceServer(srv, productrpc.NewProductRPCServer(svc))
	hs := health.NewServer()
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(srv, hs)
	return srv
}

// NewRelay 装配 Outbox Relay（业务事务内落库 → 异步 webhook 投递）。
func NewRelay(r *Repositories, cfg Config) *outbox.Relay {
	return outbox.NewRelay(r.Outbox, outbox.HTTPDeliver(cfg.EventSubscribers), cfg.RelayInterval)
}

// Run 启动 Relay（后台）+ gRPC（先）+ HTTP（后），阻塞直到 ctx 取消。
func Run(ctx context.Context, cfg Config) error {
	repos, err := NewRepositories(cfg)
	if err != nil {
		return err
	}
	svc := NewAppService(repos)

	go NewRelay(repos, cfg).Run(ctx)

	grpcSrv := NewGRPCServer(svc)
	httpSrv := NewHTTPServer(svc)

	grpcErr := make(chan error, 1)
	go func() {
		lis, err := net.Listen("tcp", cfg.GRPCAddr)
		if err != nil {
			grpcErr <- fmt.Errorf("listen gRPC %s: %w", cfg.GRPCAddr, err)
			return
		}
		slog.Info("product gRPC listening", "addr", cfg.GRPCAddr)
		grpcErr <- grpcSrv.Serve(lis)
	}()

	httpErr := make(chan error, 1)
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: httpSrv, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		slog.Info("product HTTP listening", "addr", cfg.HTTPAddr)
		httpErr <- server.ListenAndServe()
	}()

	select {
	case err := <-grpcErr:
		if err != nil && err != grpc.ErrServerStopped {
			return err
		}
	case err := <-httpErr:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
	grpcSrv.GracefulStop()
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
