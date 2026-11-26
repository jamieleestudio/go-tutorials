package application

import "context"

// OrderService 订单上下文的应用服务契约。
// 接口层只依赖本接口，永远不依赖 impl。
type OrderService interface {
	CreateOrder(ctx context.Context, cmd CreateOrderCommand) (*OrderDto, error)
	PayOrder(ctx context.Context, cmd PayOrderCommand) error
	CancelOrder(ctx context.Context, cmd CancelOrderCommand) error
	GetOrderByID(ctx context.Context, query GetOrderByIDQuery) (*OrderDto, error)
	GetOrdersByCustomer(ctx context.Context, query GetOrdersByCustomerQuery) ([]*OrderDto, error)
}
