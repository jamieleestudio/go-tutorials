// Package domain 支付限界上下文的领域层（纯领域，零框架依赖）。
package domain

import (
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared"
	"github.com/shopspring/decimal"
)

// PaymentStatus 支付状态值对象（对应 Java 的 PaymentStatus 枚举）。
type PaymentStatus string

const (
	StatusPending  PaymentStatus = "PENDING"
	StatusSuccess  PaymentStatus = "SUCCESS"
	StatusFailed   PaymentStatus = "FAILED"
	StatusRefunded PaymentStatus = "REFUNDED"
)

// Payment 支付聚合根。
// 状态机：PENDING → SUCCESS → REFUNDED；PENDING → FAILED。
type Payment struct {
	shared.AggregateRoot
	id            string
	orderID       string
	amount        decimal.Decimal
	status        PaymentStatus
	failureReason string
}

func Create(orderID string, amount decimal.Decimal) (*Payment, error) {
	if orderID == "" {
		return nil, shared.NewBusinessRuleViolation("INVALID_PAYMENT", "Order ID must not be empty")
	}
	if amount.IsZero() || amount.IsNegative() {
		return nil, shared.NewBusinessRuleViolation("INVALID_PAYMENT", "Amount must be positive")
	}
	return &Payment{id: shared.NewID(), orderID: orderID, amount: amount, status: StatusPending}, nil
}

func Reconstitute(id, orderID string, amount decimal.Decimal, status PaymentStatus, failureReason string) *Payment {
	return &Payment{id: id, orderID: orderID, amount: amount, status: status, failureReason: failureReason}
}

// Succeed 支付成功（demo 里模拟支付网关：处理即成功）。
func (p *Payment) Succeed() error {
	if p.status != StatusPending {
		return shared.NewBusinessRuleViolation("INVALID_TRANSITION",
			"Cannot succeed payment in status "+string(p.status))
	}
	p.status = StatusSuccess
	return nil
}

// Fail 支付失败。
func (p *Payment) Fail(reason string) error {
	if p.status != StatusPending {
		return shared.NewBusinessRuleViolation("INVALID_TRANSITION",
			"Cannot fail payment in status "+string(p.status))
	}
	p.status = StatusFailed
	p.failureReason = reason
	return nil
}

// Refund 退款：只有 SUCCESS 可以退。
func (p *Payment) Refund() error {
	if p.status != StatusSuccess {
		return shared.NewBusinessRuleViolation("INVALID_TRANSITION",
			"Cannot refund payment in status "+string(p.status))
	}
	p.status = StatusRefunded
	return nil
}

func (p *Payment) ID() string              { return p.id }
func (p *Payment) OrderID() string         { return p.orderID }
func (p *Payment) Amount() decimal.Decimal { return p.amount }
func (p *Payment) Status() PaymentStatus   { return p.status }
func (p *Payment) FailureReason() string   { return p.failureReason }
