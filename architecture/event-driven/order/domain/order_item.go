package domain

import (
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared"
	"github.com/shopspring/decimal"
)

// OrderItem 订单行值对象（对应 Java 的 OrderItem）。
type OrderItem struct {
	productID   string
	productName string
	quantity    int
	unitPrice   decimal.Decimal
}

func NewOrderItem(productID, productName string, quantity int, unitPrice decimal.Decimal) (*OrderItem, error) {
	if productID == "" {
		return nil, shared.NewBusinessRuleViolation("INVALID_ITEM", "Product ID must not be empty")
	}
	if quantity <= 0 {
		return nil, shared.NewBusinessRuleViolation("INVALID_ITEM", "Quantity must be positive")
	}
	if unitPrice.IsNegative() {
		return nil, shared.NewBusinessRuleViolation("INVALID_ITEM", "Unit price must not be negative")
	}
	return &OrderItem{productID: productID, productName: productName, quantity: quantity, unitPrice: unitPrice}, nil
}

// SubTotal 小计 = 单价 × 数量。
func (i *OrderItem) SubTotal() decimal.Decimal {
	return i.unitPrice.Mul(decimal.NewFromInt(int64(i.quantity)))
}

func (i *OrderItem) ProductID() string          { return i.productID }
func (i *OrderItem) ProductName() string        { return i.productName }
func (i *OrderItem) Quantity() int              { return i.quantity }
func (i *OrderItem) UnitPrice() decimal.Decimal { return i.unitPrice }
