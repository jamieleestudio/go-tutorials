// Package domain 订单限界上下文的领域层（纯领域，零框架依赖）。
package domain

import (
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared"
	"github.com/shopspring/decimal"
)

// Order 订单聚合根 —— Saga 状态机。
//
// 与 P4（同步 RPC，CREATED → PAID → CONFIRMED）不同，P5 的支付与扣库存
// 都是异步的：只有【两个】事件都到达（PaymentSucceeded AND InventoryDeducted）
// 才 CONFIRMED —— 最终一致性的教科书式落地。对应 Java P5 的 Order。
type Order struct {
	id                string
	customerID        string
	items             []*OrderItem
	status            OrderStatus
	paymentID         string
	paymentSucceeded  bool
	inventoryDeducted bool
}

func Create(customerID string, items []*OrderItem) (*Order, error) {
	if customerID == "" {
		return nil, shared.NewBusinessRuleViolation("INVALID_ORDER", "Customer ID must not be empty")
	}
	if len(items) == 0 {
		return nil, shared.NewBusinessRuleViolation("INVALID_ORDER", "Order must contain at least one item")
	}
	return &Order{
		id:         shared.NewID(),
		customerID: customerID,
		items:      items,
		status:     StatusCreated,
	}, nil
}

// Reconstitute 从持久化状态重建聚合（供仓储实现使用）。
func Reconstitute(id, customerID string, items []*OrderItem, status OrderStatus,
	paymentID string, paymentSucceeded, inventoryDeducted bool) *Order {
	return &Order{
		id:                id,
		customerID:        customerID,
		items:             items,
		status:            status,
		paymentID:         paymentID,
		paymentSucceeded:  paymentSucceeded,
		inventoryDeducted: inventoryDeducted,
	}
}

// MarkPaymentSucceeded Saga 反应：PaymentSucceededEvent 到达。
// 只有 CREATED 状态能记录（重放/迟到的事件被领域守卫拒绝）。
func (o *Order) MarkPaymentSucceeded(paymentID string) error {
	if o.status != StatusCreated {
		return shared.NewBusinessRuleViolation("INVALID_TRANSITION",
			"Cannot record payment for order in status "+string(o.status))
	}
	o.paymentID = paymentID
	o.paymentSucceeded = true
	o.confirmIfReady()
	return nil
}

// MarkInventoryDeducted Saga 反应：InventoryDeductedEvent 到达。
func (o *Order) MarkInventoryDeducted() error {
	if o.status != StatusCreated {
		return shared.NewBusinessRuleViolation("INVALID_TRANSITION",
			"Cannot record inventory for order in status "+string(o.status))
	}
	o.inventoryDeducted = true
	o.confirmIfReady()
	return nil
}

// confirmIfReady 两个异步条件都到齐 → CONFIRMED（最终一致性时刻）。
func (o *Order) confirmIfReady() {
	if o.paymentSucceeded && o.inventoryDeducted && o.status == StatusCreated {
		o.status = StatusConfirmed
	}
}

// Cancel Saga 补偿：取消订单（仅 CREATED 可取消 —— 状态机守卫）。
func (o *Order) Cancel(reason string) error {
	if !o.status.CanTransitionTo(StatusCancelled) {
		return shared.NewBusinessRuleViolation("INVALID_TRANSITION",
			"Cannot cancel order in status "+string(o.status))
	}
	_ = reason // P5 版 reason 只随 OrderCancelledEvent 发布，不落在聚合上
	o.status = StatusCancelled
	return nil
}

// TotalAmount 订单总金额 = 所有行小计之和。
func (o *Order) TotalAmount() decimal.Decimal {
	total := decimal.Zero
	for _, item := range o.items {
		total = total.Add(item.SubTotal())
	}
	return total
}

func (o *Order) ID() string              { return o.id }
func (o *Order) CustomerID() string      { return o.customerID }
func (o *Order) Status() OrderStatus     { return o.status }
func (o *Order) PaymentID() string       { return o.paymentID }
func (o *Order) PaymentSucceeded() bool  { return o.paymentSucceeded }
func (o *Order) InventoryDeducted() bool { return o.inventoryDeducted }

// Items 返回订单行的快照拷贝（对应 Java 的 unmodifiableList）。
func (o *Order) Items() []*OrderItem {
	out := make([]*OrderItem, len(o.items))
	copy(out, o.items)
	return out
}
