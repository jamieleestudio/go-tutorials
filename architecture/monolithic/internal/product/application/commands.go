// Package application 产品上下文的应用层。
package application

import (
	"github.com/shopspring/decimal"
)

// CreateProductCommand 创建产品命令（写操作）。
type CreateProductCommand struct {
	Name  string
	Price decimal.Decimal
	Stock int
}

// DeductStockCommand 扣减库存命令（写操作）。
type DeductStockCommand struct {
	ProductID string
	Quantity  int
}
