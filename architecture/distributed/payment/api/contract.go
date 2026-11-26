// Package api 是 payment 上下文的跨服务契约包（对应 Java 的 ms-payment-api 模块）。
//
// 被远程的 order 服务消费：Java 版经 OpenFeign，Go 版经 infrastructure/rpc
// 的 HTTP 客户端（端点：POST /rpc/payments/pay、POST /rpc/payments/{paymentId}/refund）。
// 铁律：契约包只依赖 shared，不依赖本服务任何内部包。
package api

import (
	"context"

	"github.com/shopspring/decimal"
)

// PaymentDto 支付单出参契约（对应 Java payment-api 的 PaymentDto record）。
type PaymentDto struct {
	PaymentID string          `json:"paymentId"`
	OrderID   string          `json:"orderId"`
	Amount    decimal.Decimal `json:"amount"`
	Status    string          `json:"status"` // PENDING / SUCCESS / FAILED / REFUNDED
}

// PaymentService 支付提供者契约 —— 只含跨上下文命令（CROSS-CONTEXT methods only）。
type PaymentService interface {
	// Pay 为订单支付。幂等：同一订单重复支付返回同一个支付单 ID
	// （分布式下 RPC 重试不能产生重复支付，对应 Java 版同语义）。
	// 返回支付单 ID。
	Pay(ctx context.Context, orderID string, amount decimal.Decimal) (string, error)

	// Refund 退款。
	Refund(ctx context.Context, paymentID string) error
}
