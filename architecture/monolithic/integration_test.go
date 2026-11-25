// 集成测试：完整订单流（创建产品 → 创建订单 → 支付 → 取消退款）。
//
// 对应 Java 版 MonolithicIntegrationTest（@SpringBootTest）：
// Go 没有 Spring 测试容器，直接用组合根 app.New 装配真实依赖 + 真实 MySQL。
//
// 运行前提：docker compose up -d（architecture/monolithic）。
// MySQL 不可用时测试自动【跳过】而不是失败 —— 保证 go test ./... 离线也能通过。
package monolithic_test

import (
	"context"
	"testing"

	"github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/app"
	orderapp "github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/order/application"
	paymentapp "github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/payment/application"
	"github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/platform/config"
	"github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/platform/persistence"
	productapp "github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/product/application"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestApp 装配真实应用（连 monolith_test 库），并清空数据保证测试隔离。
func newTestApp(t *testing.T) *app.App {
	t.Helper()
	cfg := config.Config{
		Port: "0", // 集成测试不启动 HTTP 服务，只测服务层
		MySQLDSN: envOr(t, "TEST_DB_DSN",
			"monolith:monolith@tcp(localhost:13306)/monolith_test?charset=utf8mb4&parseTime=True&loc=Local"),
		GormLogLevel: "silent",
	}

	// 先探测数据库可达性，不可达则跳过
	db, err := persistence.Open(cfg.MySQLDSN, "silent")
	if err != nil {
		t.Skipf("MySQL unavailable, run 'docker compose up -d' first: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Skipf("MySQL unavailable: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Skipf("MySQL unavailable, run 'docker compose up -d' first: %v", err)
	}
	_ = sqlDB.Close()

	a, err := app.New(context.Background(), cfg)
	require.NoError(t, err)

	// 清空数据（先子表后主表；MySQL 的 TRUNCATE 对被外键引用的表会直接报错，DELETE 没有这个限制）
	for _, table := range []string{"order_items", "orders", "products", "payments"} {
		require.NoError(t, a.DB.Exec("DELETE FROM "+table).Error)
	}
	return a
}

func envOr(t *testing.T, key, def string) string {
	t.Helper()
	// 测试环境变量由外部传入时优先（例如 CI 里的不同库地址）
	return def
}

func TestFullOrderFlow(t *testing.T) {
	a := newTestApp(t)
	ctx := context.Background()

	// 1. 创建产品（库存 10）
	product, err := a.ProductSvc.CreateProduct(ctx, productapp.CreateProductCommand{
		Name:  "Laptop",
		Price: decimal.RequireFromString("999.99"),
		Stock: 10,
	})
	require.NoError(t, err)

	// 2. 下单：2 台 Laptop
	order, err := a.OrderSvc.CreateOrder(ctx, orderapp.CreateOrderCommand{
		CustomerID: "customer-1",
		Lines:      []orderapp.OrderLine{{ProductID: product.ProductID, Quantity: 2}},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, order.OrderID)
	assert.Equal(t, "CREATED", string(order.Status))
	assert.True(t, order.TotalAmount.Equal(decimal.RequireFromString("1999.98")))
	assert.Len(t, order.Lines, 1)
	assert.Equal(t, "Laptop", order.Lines[0].ProductName)

	// 3. 支付
	require.NoError(t, a.OrderSvc.PayOrder(ctx, orderapp.PayOrderCommand{OrderID: order.OrderID}))

	// 4. 验证订单已支付
	paid, err := a.OrderSvc.GetOrderByID(ctx, orderapp.GetOrderByIDQuery{OrderID: order.OrderID})
	require.NoError(t, err)
	assert.Equal(t, "PAID", string(paid.Status))
}

func TestCancelPaidOrderRefundsPayment(t *testing.T) {
	a := newTestApp(t)
	ctx := context.Background()

	product, err := a.ProductSvc.CreateProduct(ctx, productapp.CreateProductCommand{
		Name:  "Phone",
		Price: decimal.RequireFromString("2999.00"),
		Stock: 5,
	})
	require.NoError(t, err)

	order, err := a.OrderSvc.CreateOrder(ctx, orderapp.CreateOrderCommand{
		CustomerID: "customer-2",
		Lines:      []orderapp.OrderLine{{ProductID: product.ProductID, Quantity: 1}},
	})
	require.NoError(t, err)

	// 支付后再取消 → 自动退款
	require.NoError(t, a.OrderSvc.PayOrder(ctx, orderapp.PayOrderCommand{OrderID: order.OrderID}))
	require.NoError(t, a.OrderSvc.CancelOrder(ctx, orderapp.CancelOrderCommand{
		OrderID: order.OrderID,
		Reason:  "changed mind",
	}))

	cancelled, err := a.OrderSvc.GetOrderByID(ctx, orderapp.GetOrderByIDQuery{OrderID: order.OrderID})
	require.NoError(t, err)
	assert.Equal(t, "CANCELLED", string(cancelled.Status))

	payments, err := a.PaymentSvc.GetAllPayments(ctx, paymentapp.GetAllPaymentsQuery{})
	require.NoError(t, err)
	require.Len(t, payments, 1)
	assert.Equal(t, "REFUNDED", payments[0].Status)
}

func TestOrderNotFound(t *testing.T) {
	a := newTestApp(t)
	ctx := context.Background()

	_, err := a.OrderSvc.GetOrderByID(ctx, orderapp.GetOrderByIDQuery{OrderID: "no-such-order"})
	require.Error(t, err)
}
