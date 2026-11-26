package application

import (
	"github.com/jamieleestudio/go-tutorials/architecture/microservices/order/domain"
	"github.com/shopspring/decimal"
)

// OrderDto 订单出参（接口层转换为 web Response）。
type OrderDto struct {
	OrderID     string
	CustomerID  string
	Status      domain.OrderStatus
	TotalAmount decimal.Decimal
	Lines       []OrderLineDto
}

// OrderLineDto 订单行出参。
type OrderLineDto struct {
	ProductID   string
	ProductName string
	Quantity    int
	UnitPrice   decimal.Decimal
}
