package domain

import (
	"context"
	"errors"
)

// ErrPaymentNotFound 支付单不存在的哨兵错误（由仓储实现返回，应用层转换）。
var ErrPaymentNotFound = errors.New("payment not found")

// PaymentRepository 支付聚合的仓储端口。
type PaymentRepository interface {
	Save(ctx context.Context, payment *Payment) error
	FindByID(ctx context.Context, paymentID string) (*Payment, error)
	FindByOrderID(ctx context.Context, orderID string) (*Payment, error)
	FindAll(ctx context.Context) ([]*Payment, error)
}
