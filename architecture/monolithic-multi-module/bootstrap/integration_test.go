// 集成测试：完整订单流（创建产品 → 创建订单 → 支付 → 取消退款）。
//
// 对应 Java 版 mmm-bootstrap 的 MonolithicMultiModuleIntegrationTest（@SpringBootTest + H2 内存库）：
// Go 版用组合根 bootstrap.New 装配全部模块 + SQLite 内存库 —— 零外部依赖，
// go test ./... 无需 Docker、无需 MySQL。
package bootstrap_test

import (
	"context"
	"testing"

	"github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/bootstrap"
	"github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/bootstrap/config"
	orderapp "github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/order/application"
	paymentapp "github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/payment/application"
	productapp "github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/product/application"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestApp 装配真实应用（SQLite 共享内存库），并清空数据保证测试隔离。
func newTestApp(t *testing.T) *bootstrap.App {
	t.Helper()
	cfg := config.Config{
		Port: "0", // 集成测试不启动 HTTP 服务，只测服务层
		DSN:  "sqlite://file:mmm-test?mode=memory&cache=shared",
		GormLogLevel: "silent",
	}

	a, err := bootstrap.New(context.Background(), cfg)
	require.NoError(t, err)

	// 清空数据（先子表后主表，避免外键约束）
	for _, table := range []string{"order_items", "orders", "products", "payments"} {
		require.NoError(t, a.DB.Exec("DELETE FROM "+table).Error)
	}
	return a
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
