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
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/jamieleestudio/go-tutorials/architecture/microservices/product/application/impl"
	productinf "github.com/jamieleestudio/go-tutorials/architecture/microservices/product/infrastructure"
	"github.com/jamieleestudio/go-tutorials/architecture/microservices/product/interfaces/httpmw"
	productrpc "github.com/jamieleestudio/go-tutorials/architecture/microservices/product/interfaces/rpc"
	productweb "github.com/jamieleestudio/go-tutorials/architecture/microservices/product/interfaces/web"
	"github.com/jamieleestudio/go-tutorials/architecture/microservices/product/proto/productpb"
	"github.com/jamieleestudio/go-tutorials/architecture/microservices/shared/persistence"
)

// Config 服务装配配置（对应 Java 的 application.yml）。
type Config struct {
	HTTPAddr string // REST 监听地址，默认 :8083
	GRPCAddr string // gRPC 监听地址，默认 :9183
	DSN      string // 数据库 DSN，默认 SQLite 内存库
	LogLevel string // gorm 日志级别
}

// DefaultConfig 从环境变量读配置，未设置时取与 Java 版一致的默认值。
// 服务发现留了接缝：P4 用固定地址 env（Nacos 的对应物演进见 README）。
func DefaultConfig() Config {
	return Config{
		HTTPAddr: envOr("PRODUCT_HTTP_ADDR", ":8083"),
		GRPCAddr: envOr("PRODUCT_GRPC_ADDR", ":9183"),
		DSN:      envOr("DB_DSN", "sqlite://file:product?mode=memory&cache=shared"),
		LogLevel: envOr("DB_LOG_LEVEL", "warn"),
	}
}

// NewHTTPServer 装配 REST 路由（依赖注入全部在此显式完成）。
func NewHTTPServer(repo *productinf.ProductRepositoryImpl) (*gin.Engine, error) {
	appService := impl.NewProductServiceImpl(repo)

	r := gin.New()
	r.Use(gin.Recovery(), httpmw.ErrorHandler())
	productweb.NewProductController(appService).Register(r)
	return r, nil
}

// NewGRPCServer 装配 gRPC server（pb.ProductQueryService → api.ProductService）。
func NewGRPCServer(repo *productinf.ProductRepositoryImpl) *grpc.Server {
	appService := impl.NewProductServiceImpl(repo)

	srv := grpc.NewServer()
	productpb.RegisterProductQueryServiceServer(srv, productrpc.NewProductRPCServer(appService))
	hs := health.NewServer()
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(srv, hs)
	return srv
}

// NewRepositories 打开数据库 + 建表 + 构造仓储（服务独享一个数据库）。
func NewRepositories(cfg Config) (*productinf.ProductRepositoryImpl, error) {
	db, err := persistence.Open(cfg.DSN, cfg.LogLevel)
	if err != nil {
		return nil, err
	}
	if err := persistence.Migrate(db, &productinf.ProductEntity{}); err != nil {
		return nil, fmt.Errorf("migrate product db: %w", err)
	}
	return productinf.NewProductRepository(db), nil
}

// Run 启动 gRPC（先）+ HTTP（后），阻塞直到 ctx 取消。
// 返回前对两个 server 做优雅关闭。
func Run(ctx context.Context, cfg Config) error {
	repo, err := NewRepositories(cfg)
	if err != nil {
		return err
	}

	grpcSrv := NewGRPCServer(repo)
	httpSrv, err := NewHTTPServer(repo)
	if err != nil {
		return err
	}

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
