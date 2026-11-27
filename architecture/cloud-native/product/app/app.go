// Package app product 服务的组合根（对应 Java cn-product-service 的 @SpringBootApplication）。
//
// P7 与 P6 的分工刻意保持"业务装配零修改"（对应 Java 版 ③→⑥ 业务代码逐字节相同）：
// 变化只在运行时外壳 —— 统一 :8080（K8s 容器惯例）、健康探针/指标端点、
// SIGTERM 优雅停机（设施全部在 shared/cloudkit）。
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

	"github.com/jamieleestudio/go-tutorials/architecture/cloud-native/product/application"
	"github.com/jamieleestudio/go-tutorials/architecture/cloud-native/product/application/impl"
	productinf "github.com/jamieleestudio/go-tutorials/architecture/cloud-native/product/infrastructure"
	"github.com/jamieleestudio/go-tutorials/architecture/cloud-native/product/interfaces/httpmw"
	productweb "github.com/jamieleestudio/go-tutorials/architecture/cloud-native/product/interfaces/web"
	"github.com/jamieleestudio/go-tutorials/architecture/cloud-native/shared/cloudkit"
	"github.com/jamieleestudio/go-tutorials/architecture/cloud-native/shared/persistence"
)

// Config 装配配置（对应 Java 的 application.yml —— 12-Factor：全部 ${ENV:default}）。
type Config struct {
	HTTPAddr string // REST + RPC 共用端口；K8s 惯例统一 :8080（PORT 环境变量可覆盖）
	DSN      string // 数据库 DSN，默认 SQLite 内存（对应 Java 的 H2 内存库）
	LogLevel string // gorm 日志级别
}

// DefaultConfig 从环境变量读配置（对应 Java 的 ${PORT:8080} 占位符；
// 配置由 K8s ConfigMap/Secret → env 注入，镜像本身不含环境差异）。
func DefaultConfig() Config {
	return Config{
		HTTPAddr: cloudkit.AddrFromPort("8080"),
		DSN:      cloudkit.EnvOr("DB_DSN", "sqlite://file:cn-product?mode=memory&cache=shared"),
		LogLevel: cloudkit.EnvOr("DB_LOG_LEVEL", "warn"),
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

// NewHTTPServer 装配业务路由：REST（对外端点）+ RPC（对内 order 服务调用）。
// 只含业务端点 —— 云原生运行时端点（探针/指标）由 Run 挂载，业务测试不用带设施。
func NewHTTPServer(svc application.ProductAppService) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), httpmw.ErrorHandler())
	productweb.NewProductController(svc).Register(r)
	productweb.NewProductRpcController(svc).Register(r)
	return r
}

// Run 装配并运行 product 服务，直到 ctx 取消（SIGTERM/SIGINT → 优雅停机）。
func Run(ctx context.Context, cfg Config) error {
	repos, err := NewRepositories(cfg)
	if err != nil {
		return err
	}
	svc := NewAppService(repos)
	// 云原生运行时设施（对应 Java 的 actuator 依赖 + 声明式 management 配置）。
	health := cloudkit.NewHealth("product")
	metrics := cloudkit.NewMetrics(health)

	engine := NewHTTPServer(svc)
	for _, rt := range cloudkit.Routes(health, metrics) {
		engine.Handle(rt.Method, rt.Path, gin.WrapF(rt.Handler)) // /healthz/liveness /healthz/readiness /healthz /info /metrics
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           metrics.Middleware(engine), // 指标中间件包在最外层，业务+探针统一计数
		ReadHeaderTimeout: 10 * time.Second,
	}

	// bind 先于 ready 翻转：端口开着但 readiness 503 的窗口里，K8s 不放流量进来。
	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.HTTPAddr, err)
	}

	// 装配（含 DB 迁移）完成 → 放行 readiness；SIGTERM 时再摘流。
	// K8s readinessProbe 轮询 /healthz/readiness：503 期间 pod 不进 Service endpoints。
	health.SetReady(true)
	slog.Info("product HTTP listening", "addr", ln.Addr().String(),
		"probes", "/healthz/liveness /healthz/readiness", "metrics", "/metrics")

	// 优雅停机：SIGTERM → readiness=false（摘流）→ 排水存量请求（8s 窗口）→ 关库。
	return cloudkit.Serve(ctx, health, server, ln, 8*time.Second, func() error {
		sqlDB, err := repos.DB.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	})
}
