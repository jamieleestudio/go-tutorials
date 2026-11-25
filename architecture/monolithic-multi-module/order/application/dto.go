package application

import (
	"github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/order/domain"
	"github.com/shopspring/decimal"
)

// OrderLineDto 订单行出参。
type OrderLineDto struct {
	ProductID   string
	ProductName string
	Quantity    int
	UnitPrice   decimal.Decimal
}

// OrderDto 订单出参（应用层输出契约）。
// 供接口层转换为 Web Response；也供跨上下文消费。
// 注意：Java 版 DTO 直接引用了领域枚举 OrderStatus，这里保持一致。
type OrderDto struct {
	OrderID     string
	CustomerID  string
	Status      domain.OrderStatus
	TotalAmount decimal.Decimal
	Lines       []OrderLineDto
}
