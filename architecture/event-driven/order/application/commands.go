// Package application 订单上下文的应用层：编排用例、管理事务边界。
package application

// CreateOrderCommand 创建订单命令（写操作）。
type CreateOrderCommand struct {
	CustomerID string
	Lines      []OrderLine
}

// OrderLine 下单行输入（跨服务契约的输入形态）。
type OrderLine struct {
	ProductID string
	Quantity  int
}

// CancelOrderCommand 取消订单命令（写操作 → Saga 补偿起点）。
type CancelOrderCommand struct {
	OrderID string
	Reason  string
}
