// Package app 组合根（Composition Root）。
//
// 对应 Java 版 Spring 的 @SpringBootApplication 组件扫描 + 依赖注入：
// Go 没有 IoC 容器，所有依赖关系在这里【手动装配】—— 每一行都是"new 对象 + 注入"。
// 看懂本文件，就看懂了整个应用的依赖图。
package app

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	orderapp "github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/order/application"
	orderappimpl "github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/order/application/impl"
	orderevent "github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/order/interfaces/event"
	orderweb "github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/order/interfaces/web"
	orderinfra "github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/order/infrastructure"
	paymentapp "github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/payment/application"
	paymentappimpl "github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/payment/application/impl"
	paymentweb "github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/payment/interfaces/web"
	paymentinfra "github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/payment/infrastructure"
	"github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/platform/config"
	"github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/platform/httpmw"
	"github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/platform/persistence"
	productapp "github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/product/application"
	productappimpl "github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/product/application/impl"
	productweb "github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/product/interfaces/web"
	productinfra "github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/product/infrastructure"
	"github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/shared"
	"gorm.io/gorm"
)

// App 装配完成的单体应用：数据库、事件总线、HTTP 引擎与各上下文服务。
type App struct {
	DB         *gorm.DB
	Bus        *shared.EventBus
	Engine     *gin.Engine
	ProductSvc productapp.ProductService
	PaymentSvc paymentapp.PaymentService
	OrderSvc   orderapp.OrderService
}

// New 装配整个应用（等价于 Spring 启动时做的所有事）。
func New(_ context.Context, cfg config.Config) (*App, error) {
	// 1. 数据库 + 建表（对应 spring.datasource + ddl-auto）
	db, err := persistence.Open(cfg.MySQLDSN, cfg.GormLogLevel)
	if err != nil {
		return nil, err
	}
	if err := persistence.Migrate(db,
		&orderinfra.OrderEntity{}, &orderinfra.OrderItemEntity{},
		&productinfra.ProductEntity{}, &paymentinfra.PaymentEntity{},
	); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}

	// 2. 平台设施：事件总线（ApplicationEventPublisher）+ 事务运行器（@Transactional）
	bus := shared.NewEventBus()
	txRunner := persistence.NewGormTxRunner(db)

	// 3. Product 上下文：仓储 → 服务
	productRepo := productinfra.NewProductRepository(db)
	var productSvc productapp.ProductService = productappimpl.NewProductService(productRepo, txRunner)

	// 4. Payment 上下文
	paymentRepo := paymentinfra.NewPaymentRepository(db)
	var paymentSvc paymentapp.PaymentService = paymentappimpl.NewPaymentService(paymentRepo, txRunner)

	// 5. Order 上下文：注意只注入 Product/Payment 的【应用层接口】
	eventPublisher := orderinfra.NewEventPublisher(bus)
	orderRepo := orderinfra.NewOrderRepository(db)
	var orderSvc orderapp.OrderService = orderappimpl.NewOrderService(
		orderRepo, productSvc, paymentSvc, eventPublisher, txRunner)

	// 6. 事件监听器订阅事件总线（对应 @EventListener）
	orderevent.RegisterHandlers(bus)

	// 7. HTTP 引擎 + 路由注册（对应 @RestController 自动映射）
	engine := gin.New()
	engine.Use(gin.Logger(), gin.Recovery(), httpmw.ErrorHandler())
	productweb.NewProductController(productSvc).Register(engine)
	paymentweb.NewPaymentController(paymentSvc).Register(engine)
	orderweb.NewOrderController(orderSvc).Register(engine)

	return &App{
		DB:         db,
		Bus:        bus,
		Engine:     engine,
		ProductSvc: productSvc,
		PaymentSvc: paymentSvc,
		OrderSvc:   orderSvc,
	}, nil
}
