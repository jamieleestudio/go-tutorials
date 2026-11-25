// Package domain 支付限界上下文的领域层。
//
// 对应 Java 的 com.example.monolithic.payment.domain 包。
package domain

import (
	"github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/shared"
	"github.com/shopspring/decimal"
)

// PaymentStatus 支付状态值对象。
type PaymentStatus string

const (
	StatusPending  PaymentStatus = "PENDING"
	StatusSuccess  PaymentStatus = "SUCCESS"
	StatusFailed   PaymentStatus = "FAILED"
	StatusRefunded PaymentStatus = "REFUNDED"
)

// Payment 支付聚合根。
// 嵌入 shared.AggregateRoot（Java 版也继承了 AggregateRoot，但未发布事件，
// 这里保持一致以便对照学习）。
type Payment struct {
	shared.AggregateRoot
	id            string
	orderID       string
	amount        decimal.Decimal
	status        PaymentStatus
	failureReason string
}

// Create 工厂方法：新支付单从 PENDING 开始。
func Create(orderID string, amount decimal.Decimal) (*Payment, error) {
	if orderID == "" {
		return nil, shared.NewBusinessRuleViolation("INVALID_PAYMENT", "Order ID must not be empty")
	}
	if amount.IsZero() || amount.IsNegative() {
		return nil, shared.NewBusinessRuleViolation("INVALID_PAYMENT", "Amount must be positive")
	}
	return &Payment{
		id:      shared.NewID(),
		orderID: orderID,
		amount:  amount,
		status:  StatusPending,
	}, nil
}

// Reconstitute 从持久化状态重建聚合（仓储实现专用）。
func Reconstitute(id, orderID string, amount decimal.Decimal, status PaymentStatus, failureReason string) *Payment {
	return &Payment{
		id:            id,
		orderID:       orderID,
		amount:        amount,
		status:        status,
		failureReason: failureReason,
	}
}
