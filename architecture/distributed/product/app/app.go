// Package app product 服务的组合根（对应 Java dist-product-service 的 @SpringBootApplication）。
//
// 装配逻辑不放在 cmd/main.go 里，原因同前几期：
//  1. e2e 测试可以直接在进程内按随机端口装配整个服务（对应 @SpringBootTest）；
//  2. cmd/main.go 保持"只读配置 + 调 Run"的最小静态。
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

	"github.com/jamieleestudio/go-tutorials/architecture/distributed/product/application"
	"github.com/jamieleestudio/go-tutorials/architecture/distributed/product/application/impl"
	productinf "github.com/jamieleestudio/go-tutorials/architecture/distributed/product/infrastructure"
	"github.com/jamieleestudio/go-tutorials/architecture/distributed/product/interfaces/httpmw"
	productweb "github.com/jamieleestudio/go-tutorials/architecture/distributed/product/interfaces/web"
	"github.com/jamieleestudio/go-tutorials/architecture/distributed/shared/persistence"
)

// Config 装配配置（对应 Java 的 application.yml）。
type Config struct {
	HTTPAddr string // REST + RPC 共用端口，默认 :8083
	DSN      string // 数据库 DSN，默认 SQLite 内存
	LogLevel string // gorm 日志级别
}

// DefaultConfig 从环境变量读配置。P6 无注册中心：端口即地址。
func DefaultConfig() Config {
	return Config{
		HTTPAddr: envOr("PRODUCT_HTTP_ADDR", ":8083"),
		DSN:      envOr("DB_DSN", "sqlite://file:dist-product?mode=memory&cache=shared"),
		LogLevel: envOr("DB_LOG_LEVEL", "warn"),
	}
}

// Repositories 全局持久化设施（一个服务一个数据库）。
type Repositories struct {
	DB       *gorm.DB
	Products *productinf.ProductRepositoryImpl
}

// NewRepositories 打开数据库 + 迁移 + 构造仓储。
func NewRepositories(cfg Config) (*Repositories, error) {
	db, err := persistence.Open(cfg.DSN, cfg.LogLevel)
	if err != nil {
		return nil, err
	}
	if err := persistence.Migrate(db, &productinf.ProductEntity{}); err != nil {
		return nil, fmt.Errorf("migrate product db: %w", err)
	}
	return &Repositories{DB: db, Products: productinf.NewProductRepository(db)}, nil
}

// NewAppService 装配应用服务（provider-defined api 契约的真正实现者）。
func NewAppService(r *Repositories) application.ProductAppService {
	return impl.NewProductServiceImpl(r.Products)
}

// NewHTTPServer 装配路由：REST（对外业务端点）+ RPC（对内 order 服务调用）。
func NewHTTPServer(svc application.ProductAppService) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), httpmw.ErrorHandler())
	productweb.NewProductController(svc).Register(r)
	productweb.NewProductRpcController(svc).Register(r)
	return r
}

// Run 装配并运行 product 服务，直到 ctx 取消。
func Run(ctx context.Context, cfg Config) error {
	repos, err := NewRepositories(cfg)
	if err != nil {
		return err
	}
	svc := NewAppService(repos)

	httpErr := make(chan error, 1)
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: NewHTTPServer(svc), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		slog.Info("product HTTP listening", "addr", cfg.HTTPAddr)
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
