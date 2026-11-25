package application

import "github.com/shopspring/decimal"

// PaymentDto 支付单出参（应用层输出契约，供跨上下文与接口层消费）。
type PaymentDto struct {
	PaymentID string
	OrderID   string
	Amount    decimal.Decimal
	Status    string
}
