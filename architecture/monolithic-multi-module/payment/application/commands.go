// Package application 支付上下文的应用层。
package application

import (
	"github.com/shopspring/decimal"
)

// CreatePaymentCommand 创建支付单命令（写操作）。
type CreatePaymentCommand struct {
	OrderID string
	Amount  decimal.Decimal
}

// ProcessPaymentCommand 处理支付命令（写操作）。
type ProcessPaymentCommand struct {
	PaymentID string
}
