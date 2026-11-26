// Package domain 订单限界上下文的领域层（纯领域，零框架依赖）。
package domain

import (
	"time"

	"github.com/jamieleestudio/go-tutorials/architecture/distributed/order/domain/event"
	"github.com/jamieleestudio/go-tutorials/architecture/distributed/shared"
	"github.com/shopspring/decimal"
)

// Order 订单聚合根。
// 承载核心业务逻辑与状态流转；纯领域结构，不依赖任何框架。
type Order struct {
	shared.AggregateRoot
	id         string
	customerID string
	items      []*OrderItem
	status     OrderStatus
	paymentID  string
}

// Create 工厂方法：创建新订单并记录 OrderCreatedEvent。
func Create(customerID string, items []*OrderItem) (*Order, error) {
	if customerID == "" {
		return nil, shared.NewBusinessRuleViolation("INVALID_ORDER", "Customer ID must not be empty")
	}
	if len(items) == 0 {
		return nil, shared.NewBusinessRuleViolation("INVALID_ORDER", "Order must contain at least one item")
	}

	orderID := shared.NewID()
	order := &Order{id: orderID, customerID: customerID, items: items, status: StatusCreated}

	productIDs := make([]string, 0, len(items))
	total := decimal.Zero
	for _, item := range items {
		productIDs = append(productIDs, item.ProductID())
		total = total.Add(item.SubTotal())
	}
	order.RaiseEvent(&event.OrderCreatedEvent{
		OrderID:     orderID,
		ProductIDs:  productIDs,
		TotalAmount: total,
		Timestamp:   time.Now().UnixMilli(),
	})
	return order, nil
}

// Reconstitute 从持久化状态重建聚合（供仓储实现使用）。
func Reconstitute(id, customerID string, items []*OrderItem, status OrderStatus, paymentID string) *Order {
	return &Order{id: id, customerID: customerID, items: items, status: status, paymentID: paymentID}
}

// MarkAsPaid 标记订单为已支付（记录支付单 ID 并发出 OrderPaidEvent）。
func (o *Order) MarkAsPaid(paymentID string) error {
	if !o.status.CanTransitionTo(StatusPaid) {
		return shared.NewBusinessRuleViolation("INVALID_TRANSITION",
			"Cannot pay order in status "+string(o.status))
	}
	o.paymentID = paymentID
	o.status = StatusPaid
	o.RaiseEvent(&event.OrderPaidEvent{OrderID: o.id, PaymentID: paymentID, Timestamp: time.Now().UnixMilli()})
	return nil
}

// Cancel 取消订单（发出 OrderCancelledEvent）。
func (o *Order) Cancel(reason string) error {
	if !o.status.CanTransitionTo(StatusCancelled) {
		return shared.NewBusinessRuleViolation("INVALID_TRANSITION",
			"Cannot cancel order in status "+string(o.status))
	}
	o.status = StatusCancelled
	o.RaiseEvent(&event.OrderCancelledEvent{OrderID: o.id, Reason: reason, Timestamp: time.Now().UnixMilli()})
	return nil
}

// Confirm 确认订单（支付核验 + 库存扣减之后）。
func (o *Order) Confirm() error {
	if !o.status.CanTransitionTo(StatusConfirmed) {
		return shared.NewBusinessRuleViolation("INVALID_TRANSITION",
			"Cannot confirm order in status "+string(o.status))
	}
	o.status = StatusConfirmed
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

func (o *Order) ID() string          { return o.id }
func (o *Order) CustomerID() string  { return o.customerID }
func (o *Order) Status() OrderStatus { return o.status }
func (o *Order) PaymentID() string   { return o.paymentID }

// Items 返回订单行的快照拷贝（对应 Java 的 unmodifiableList）。
func (o *Order) Items() []*OrderItem {
	out := make([]*OrderItem, len(o.items))
	copy(out, o.items)
	return out
}
