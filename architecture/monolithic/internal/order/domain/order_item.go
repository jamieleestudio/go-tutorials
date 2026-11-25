package domain

import (
	"github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/shared"
	"github.com/shopspring/decimal"
)

// OrderItem 订单行值对象（Value Object）。
//
// Java 版用 BigDecimal；Go 用 shopspring/decimal（最接近 BigDecimal 的库）。
// 字段全部私有 + 构造函数校验，保证创建出的实例永远合法。
type OrderItem struct {
	productID   string
	productName string
	quantity    int
	unitPrice   decimal.Decimal
}

// NewOrderItem 构造函数：内联业务规则校验（对应 Java 的构造函数抛异常）。
func NewOrderItem(productID, productName string, quantity int, unitPrice decimal.Decimal) (OrderItem, error) {
	if productID == "" {
		return OrderItem{}, shared.NewBusinessRuleViolation("INVALID_ITEM", "Product ID must not be empty")
	}
	if quantity <= 0 {
		return OrderItem{}, shared.NewBusinessRuleViolation("INVALID_ITEM", "Quantity must be positive")
	}
	if unitPrice.IsNegative() {
		return OrderItem{}, shared.NewBusinessRuleViolation("INVALID_ITEM", "Unit price must not be negative")
	}
	return OrderItem{
		productID:   productID,
		productName: productName,
		quantity:    quantity,
		unitPrice:   unitPrice,
	}, nil
}

// SubTotal 小计 = 单价 × 数量。
func (i OrderItem) SubTotal() decimal.Decimal {
	return i.unitPrice.Mul(decimal.NewFromInt(int64(i.quantity)))
}

func (i OrderItem) ProductID() string            { return i.productID }
func (i OrderItem) ProductName() string          { return i.productName }
func (i OrderItem) Quantity() int                { return i.quantity }
func (i OrderItem) UnitPrice() decimal.Decimal   { return i.unitPrice }
