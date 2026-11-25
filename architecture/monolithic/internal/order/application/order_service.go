package application

import "context"

// OrderService 订单应用服务契约。
//
// 被【本上下文的接口层】和【基础设施的装配代码】消费；
// 与 Java 版一一对应。Go 接口方法名不带 get 前缀（惯例）。
type OrderService interface {
	CreateOrder(ctx context.Context, cmd CreateOrderCommand) (*OrderDto, error)
	PayOrder(ctx context.Context, cmd PayOrderCommand) error
	CancelOrder(ctx context.Context, cmd CancelOrderCommand) error
	GetOrderByID(ctx context.Context, query GetOrderByIDQuery) (*OrderDto, error)
	GetOrdersByCustomer(ctx context.Context, query GetOrdersByCustomerQuery) ([]OrderDto, error)
}
