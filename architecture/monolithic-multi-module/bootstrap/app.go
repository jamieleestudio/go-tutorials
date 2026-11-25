// Package bootstrap 组合根（Composition Root）—— 部署单元，组装全部业务模块。
//
// 对应 Java 版 mmm-bootstrap 的 @SpringBootApplication(scanBasePackages = "com.example.mmm")：
// Java 靠注解 + 反射在运行时发现并装配所有 Maven 模块里的组件；
// Go 在这里【手动装配】—— 每一行都是"new 对象 + 注入"，依赖图一目了然。
//
// 与 P1 的差别：所有类型都来自【其他 Go module】—— 模块边界是编译期强制的，
// bootstrap 的 go.mod require 了全部 4 个模块（这是部署单元的特权）。
package bootstrap

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	orderapp "github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/order/application"
	orderappimpl "github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/order/application/impl"
	orderevent "github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/order/interfaces/event"
	orderweb "github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/order/interfaces/web"
	orderinfra "github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/order/infrastructure"
	paymentapp "github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/payment/application"
	paymentappimpl "github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/payment/application/impl"
	paymentweb "github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/payment/interfaces/web"
	paymentinfra "github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/payment/infrastructure"
	"github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/product/application"
	productappimpl "github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/product/application/impl"
	productweb "github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/product/interfaces/web"
	productinfra "github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/product/infrastructure"
	"github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/shared"
	sharedpersistence "github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/shared/persistence"
	"github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/bootstrap/config"
	"github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/bootstrap/httpmw"
	"github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/bootstrap/persistence"
	"gorm.io/gorm"
)

// App 装配完成的应用：数据库、事件总线、HTTP 引擎与各上下文服务。
type App struct {
	DB         *gorm.DB
	Bus        *shared.EventBus
	Engine     *gin.Engine
	ProductSvc application.ProductService
	PaymentSvc paymentapp.PaymentService
	OrderSvc   orderapp.OrderService
}

// New 装配整个应用（等价于 Spring 启动时做的所有事）。
func New(_ context.Context, cfg config.Config) (*App, error) {
	// 1. 数据库 + 建表（对应 spring.datasource + ddl-auto）
	db, err := persistence.Open(cfg.DSN, cfg.GormLogLevel)
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
	//    均来自 shared 模块 —— Go 版把 Spring 提供的这两个原语做成了可复用的模块
	bus := shared.NewEventBus()
	txRunner := sharedpersistence.NewGormTxRunner(db)

	// 3. Product 模块：仓储 → 服务
	productRepo := productinfra.NewProductRepository(db)
	var productSvc application.ProductService = productappimpl.NewProductService(productRepo, txRunner)

	// 4. Payment 模块
	paymentRepo := paymentinfra.NewPaymentRepository(db)
	var paymentSvc paymentapp.PaymentService = paymentappimpl.NewPaymentService(paymentRepo, txRunner)

	// 5. Order 模块：注意只注入 Product/Payment 的【应用层接口】
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
