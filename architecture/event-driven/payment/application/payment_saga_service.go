// Package application 支付上下文的应用层：编排用例、管理事务边界。
package application

import (
	"context"

	"github.com/shopspring/decimal"
)

// PaymentSagaService Saga 的支付侧：响应 order 事件，发布 payment 事件。
// （对应 Java 的 PaymentSagaService —— payment 在 P5 是纯事件进程，
// 没有 REST 业务端点。）
type PaymentSagaService interface {
	// ProcessOrderPayment Saga 反应：OrderCreatedEvent → 支付 → 发布 PaymentSucceededEvent。
	// 按 orderId 幂等：同一订单的重复事件得到同一笔支付。
	ProcessOrderPayment(ctx context.Context, eventID string, orderID string, amount decimal.Decimal) error

	// RefundOrderPayment Saga 补偿反应：OrderCancelledEvent → 退款。
	RefundOrderPayment(ctx context.Context, eventID string, orderID string) error
}
