// Package infrastructure 订单上下文的基础设施层（GORM 持久化实现）。
package infrastructure

import (
	"time"

	"github.com/shopspring/decimal"
)

// OrderEntity 订单持久化实体（对应 Java 的 OrderEntity @Entity）。
type OrderEntity struct {
	ID         string `gorm:"column:order_id;size:36;primaryKey"`
	CustomerID string `gorm:"column:customer_id;size:36;not null;index"`
	Status     string `gorm:"column:status;size:16;not null"`
	PaymentID  string `gorm:"column:payment_id;size:36"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (OrderEntity) TableName() string { return "orders" }

// OrderItemEntity 订单行持久化实体（对应 Java 的 @ElementCollection order_items 表）。
type OrderItemEntity struct {
	ID          uint            `gorm:"primaryKey;autoIncrement"`
	OrderID     string          `gorm:"column:order_id;size:36;not null;index"`
	ProductID   string          `gorm:"column:product_id;size:36;not null"`
	ProductName string          `gorm:"column:product_name;size:128;not null"`
	Quantity    int             `gorm:"not null"`
	UnitPrice   decimal.Decimal `gorm:"column:unit_price;type:decimal(19,4);not null"`
}

func (OrderItemEntity) TableName() string { return "order_items" }
