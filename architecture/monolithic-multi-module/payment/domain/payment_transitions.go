package domain

import (
	"fmt"

	"github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/shared"
	"github.com/shopspring/decimal"
)

// --- 支付状态迁移（状态机） ---

// Succeed 支付成功：PENDING → SUCCESS。
func (p *Payment) Succeed() error {
	if p.status != StatusPending {
		return shared.NewBusinessRuleViolation(
			"INVALID_TRANSITION", fmt.Sprintf("Cannot succeed payment in status %s", p.status))
	}
	p.status = StatusSuccess
	return nil
}

// Fail 支付失败：PENDING → FAILED。
func (p *Payment) Fail(reason string) error {
	if p.status != StatusPending {
		return shared.NewBusinessRuleViolation(
			"INVALID_TRANSITION", fmt.Sprintf("Cannot fail payment in status %s", p.status))
	}
	p.status = StatusFailed
	p.failureReason = reason
	return nil
}

// Refund 退款：SUCCESS → REFUNDED。
func (p *Payment) Refund() error {
	if p.status != StatusSuccess {
		return shared.NewBusinessRuleViolation(
			"INVALID_TRANSITION", fmt.Sprintf("Cannot refund payment in status %s", p.status))
	}
	p.status = StatusRefunded
	return nil
}

// --- 访问器 ---

func (p *Payment) ID() string              { return p.id }
func (p *Payment) OrderID() string         { return p.orderID }
func (p *Payment) Amount() decimal.Decimal { return p.amount }
func (p *Payment) Status() PaymentStatus   { return p.status }
func (p *Payment) FailureReason() string   { return p.failureReason }
