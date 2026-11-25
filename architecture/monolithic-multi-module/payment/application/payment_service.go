package application

import (
	"context"

	"github.com/shopspring/decimal"
)

// PaymentService 支付应用服务契约。
type PaymentService interface {
	CreatePayment(ctx context.Context, cmd CreatePaymentCommand) (*PaymentDto, error)
	ProcessPayment(ctx context.Context, cmd ProcessPaymentCommand) (*PaymentDto, error)
	GetPaymentByID(ctx context.Context, query GetPaymentByIDQuery) (*PaymentDto, error)
	GetAllPayments(ctx context.Context, query GetAllPaymentsQuery) ([]PaymentDto, error)

	// --- 跨上下文方法（供 Order 上下文调用） ---

	// Pay 为订单支付并返回支付单 ID（demo 中模拟为必然成功）。
	Pay(ctx context.Context, orderID string, amount decimal.Decimal) (string, error)
	// Refund 退款。
	Refund(ctx context.Context, paymentID string) error
}
