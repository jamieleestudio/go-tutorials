// Package infrastructure 产品上下文的基础设施层（GORM 持久化实现）。
package infrastructure

import (
	"time"

	"github.com/shopspring/decimal"
)

// ProductEntity 产品持久化实体（对应 Java 的 ProductEntity）。
type ProductEntity struct {
	ID        string          `gorm:"column:product_id;size:36;primaryKey"`
	Name      string          `gorm:"column:name;size:128;not null"`
	Price     decimal.Decimal `gorm:"column:price;type:decimal(19,4);not null"`
	Stock     int             `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (ProductEntity) TableName() string { return "products" }
