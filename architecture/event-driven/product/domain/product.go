// Package domain 产品限界上下文的领域层（纯领域，零框架依赖）。
package domain

import (
	"fmt"

	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared"
	"github.com/shopspring/decimal"
)

// Product 产品聚合根（无领域事件的简单聚合）。
type Product struct {
	id    string
	name  string
	price decimal.Decimal
	stock int
}

func Create(name string, price decimal.Decimal, stock int) (*Product, error) {
	if name == "" {
		return nil, shared.NewBusinessRuleViolation("INVALID_PRODUCT", "Product name must not be empty")
	}
	if price.IsZero() || price.IsNegative() {
		return nil, shared.NewBusinessRuleViolation("INVALID_PRODUCT", "Price must be positive")
	}
	if stock < 0 {
		return nil, shared.NewBusinessRuleViolation("INVALID_PRODUCT", "Stock must not be negative")
	}
	return &Product{id: shared.NewID(), name: name, price: price, stock: stock}, nil
}

func Reconstitute(id, name string, price decimal.Decimal, stock int) *Product {
	return &Product{id: id, name: name, price: price, stock: stock}
}

// DeductStock 扣减库存（OUT_OF_STOCK 是最重要的业务规则之一）。
func (p *Product) DeductStock(quantity int) error {
	if quantity <= 0 {
		return shared.NewBusinessRuleViolation("INVALID_QUANTITY", "Quantity must be positive")
	}
	if p.stock < quantity {
		return shared.NewBusinessRuleViolation("OUT_OF_STOCK",
			fmt.Sprintf("Insufficient stock for product %s: available=%d, requested=%d", p.id, p.stock, quantity))
	}
	p.stock -= quantity
	return nil
}

func (p *Product) Restock(quantity int) error {
	if quantity <= 0 {
		return shared.NewBusinessRuleViolation("INVALID_QUANTITY", "Restock quantity must be positive")
	}
	p.stock += quantity
	return nil
}

func (p *Product) ChangePrice(newPrice decimal.Decimal) error {
	if newPrice.IsZero() || newPrice.IsNegative() {
		return shared.NewBusinessRuleViolation("INVALID_PRICE", "Price must be positive")
	}
	p.price = newPrice
	return nil
}

func (p *Product) ID() string             { return p.id }
func (p *Product) Name() string           { return p.name }
func (p *Product) Price() decimal.Decimal { return p.price }
func (p *Product) Stock() int             { return p.stock }
