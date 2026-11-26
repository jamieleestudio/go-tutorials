// Package app payment 服务的组合根（对应 Java 的 @SpringBootApplication）。
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

	"github.com/jamieleestudio/go-tutorials/architecture/microservices/payment/application/impl"
	paymentinf "github.com/jamieleestudio/go-tutorials/architecture/microservices/payment/infrastructure"
	"github.com/jamieleestudio/go-tutorials/architecture/microservices/payment/interfaces/httpmw"
	paymentrpc "github.com/jamieleestudio/go-tutorials/architecture/microservices/payment/interfaces/rpc"
	paymentweb "github.com/jamieleestudio/go-tutorials/architecture/microservices/payment/interfaces/web"
	"github.com/jamieleestudio/go-tutorials/architecture/microservices/payment/proto/paymentpb"
	"github.com/jamieleestudio/go-tutorials/architecture/microservices/shared/persistence"
)

// Config 服务装配配置（对应 Java 的 application.yml）。
type Config struct {
	HTTPAddr string // REST 监听地址，默认 :8082
	GRPCAddr string // gRPC 监听地址，默认 :9182
	DSN      string // 数据库 DSN，默认 SQLite 内存库
	LogLevel string
}

// DefaultConfig 从环境变量读配置。
func DefaultConfig() Config {
	return Config{
		HTTPAddr: envOr("PAYMENT_HTTP_ADDR", ":8082"),
		GRPCAddr: envOr("PAYMENT_GRPC_ADDR", ":9182"),
		DSN:      envOr("DB_DSN", "sqlite://file:payment?mode=memory&cache=shared"),
		LogLevel: envOr("DB_LOG_LEVEL", "warn"),
	}
}

// NewHTTPServer 装配 REST 路由。
func NewHTTPServer(repo *paymentinf.PaymentRepositoryImpl) (*gin.Engine, error) {
	appService := impl.NewPaymentServiceImpl(repo)

	r := gin.New()
	r.Use(gin.Recovery(), httpmw.ErrorHandler())
	paymentweb.NewPaymentController(appService).Register(r)
	return r, nil
}

// NewGRPCServer 装配 gRPC server（pb.PaymentCommandService → api.PaymentService）。
func NewGRPCServer(repo *paymentinf.PaymentRepositoryImpl) *grpc.Server {
	appService := impl.NewPaymentServiceImpl(repo)

	srv := grpc.NewServer()
	paymentpb.RegisterPaymentCommandServiceServer(srv, paymentrpc.NewPaymentRPCServer(appService))
	hs := health.NewServer()
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(srv, hs)
	return srv
}

// NewRepositories 打开数据库 + 建表 + 构造仓储。
func NewRepositories(cfg Config) (*paymentinf.PaymentRepositoryImpl, error) {
	db, err := persistence.Open(cfg.DSN, cfg.LogLevel)
	if err != nil {
		return nil, err
	}
	if err := persistence.Migrate(db, &paymentinf.PaymentEntity{}); err != nil {
		return nil, fmt.Errorf("migrate payment db: %w", err)
	}
	return paymentinf.NewPaymentRepository(db), nil
}

// Run 启动 gRPC（先）+ HTTP（后），阻塞直到 ctx 取消。
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
		slog.Info("payment gRPC listening", "addr", cfg.GRPCAddr)
		grpcErr <- grpcSrv.Serve(lis)
	}()

	httpErr := make(chan error, 1)
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: httpSrv, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		slog.Info("payment HTTP listening", "addr", cfg.HTTPAddr)
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
