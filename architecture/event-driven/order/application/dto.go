package application

import (
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/order/domain"
	"github.com/shopspring/decimal"
)

// OrderDto 订单出参（接口层转换为 web Response）。
// Saga 进度（PaymentSucceeded/InventoryDeducted）直接暴露 ——
// 让调用者看到"最终一致性进行到哪一步了"。
type OrderDto struct {
	OrderID           string
	CustomerID        string
	Status            domain.OrderStatus
	TotalAmount       decimal.Decimal
	PaymentSucceeded  bool
	InventoryDeducted bool
	Lines             []OrderLineDto
}

// OrderLineDto 订单行出参。
type OrderLineDto struct {
	ProductID   string
	ProductName string
	Quantity    int
	UnitPrice   decimal.Decimal
}
