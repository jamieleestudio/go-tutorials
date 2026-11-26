package application

import (
	"context"
)

// OrderService 自有用例 + Saga 反应处理器契约
// （对应 Java 的 OrderService —— REST 用例 + Kafka 消费者调用的事件反应）。
type OrderService interface {
	// --- 自有用例（REST 接口层消费）---

	CreateOrder(ctx context.Context, cmd CreateOrderCommand) (*OrderDto, error)
	CancelOrder(ctx context.Context, cmd CancelOrderCommand) error
	GetOrderByID(ctx context.Context, query GetOrderByIDQuery) (*OrderDto, error)
	GetOrdersByCustomer(ctx context.Context, query GetOrdersByCustomerQuery) ([]*OrderDto, error)

	// --- Saga 反应处理器（messaging 接口层消费）---

	// OnPaymentSucceeded payment-events 流：双条件之一到齐。
	OnPaymentSucceeded(ctx context.Context, eventID string, orderID string, paymentID string) error

	// OnPaymentFailed payment-events 流：支付失败 → 取消订单（补偿起点）。
	OnPaymentFailed(ctx context.Context, eventID string, orderID string, reason string) error

	// OnInventoryDeducted product-events 流：双条件之二到齐。
	OnInventoryDeducted(ctx context.Context, eventID string, orderID string) error

	// OnInventoryDeductFailed product-events 流：扣减失败（如缺货）→ 取消订单。
	OnInventoryDeductFailed(ctx context.Context, eventID string, orderID string, reason string) error
}
