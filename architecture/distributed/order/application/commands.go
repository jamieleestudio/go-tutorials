// Package application 订单上下文的应用层：编排用例、管理事务边界。
package application

// CreateOrderCommand 创建订单命令（写操作）。
type CreateOrderCommand struct {
	CustomerID string
	Lines      []OrderLine
}

// OrderLine 订单行输入（productId + quantity）。
type OrderLine struct {
	ProductID string
	Quantity  int
}

// PayOrderCommand 支付订单命令（写操作）。
type PayOrderCommand struct {
	OrderID string
}

// CancelOrderCommand 取消订单命令（写操作）。
type CancelOrderCommand struct {
	OrderID string
	Reason  string
}
