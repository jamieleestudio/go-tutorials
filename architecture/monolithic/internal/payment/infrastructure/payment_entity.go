// Package infrastructure 支付上下文的基础设施层（GORM 持久化实现）。
package infrastructure

import (
	"time"

	"github.com/shopspring/decimal"
)

// PaymentEntity 支付单持久化实体（对应 Java 的 PaymentEntity）。
type PaymentEntity struct {
	ID            string          `gorm:"column:payment_id;size:36;primaryKey"`
	OrderID       string          `gorm:"column:order_id;size:36;not null;index"`
	Amount        decimal.Decimal `gorm:"column:amount;type:decimal(19,4);not null"`
	Status        string          `gorm:"column:status;size:16;not null"`
	FailureReason string          `gorm:"column:failure_reason;size:255"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (PaymentEntity) TableName() string { return "payments" }
