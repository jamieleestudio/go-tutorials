package domain

import (
	"fmt"
	"time"

	"github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/order/domain/event"
	"github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/shared"
	"github.com/shopspring/decimal"
)

// Order 订单聚合根（Aggregate Root）。
//
// 包含核心业务逻辑与状态迁移，是纯领域对象 —— 不依赖 gin / gorm / web 框架。
// 通过嵌入 shared.AggregateRoot 获得领域事件的收集能力（模拟 Java 的 extends）。
type Order struct {
	shared.AggregateRoot
	id         string
	customerID string
	items      []OrderItem
	status     OrderStatus
	paymentID  string
}

// Create 工厂方法：创建新订单，并记录 OrderCreatedEvent。
func Create(customerID string, items []OrderItem) (*Order, error) {
	if customerID == "" {
		return nil, shared.NewBusinessRuleViolation("INVALID_ORDER", "Customer ID must not be empty")
	}
	if len(items) == 0 {
		return nil, shared.NewBusinessRuleViolation("INVALID_ORDER", "Order must contain at least one item")
	}

	productIDs := make([]string, 0, len(items))
	total := decimal.Zero
	for _, it := range items {
		productIDs = append(productIDs, it.ProductID())
		total = total.Add(it.SubTotal())
	}

	o := &Order{
		id:         shared.NewID(),
		customerID: customerID,
		items:      append([]OrderItem(nil), items...),
		status:     StatusCreated,
	}
	o.RaiseEvent(event.OrderCreatedEvent{
		OrderID:     o.id,
		ProductIDs:  productIDs,
		TotalAmount: total,
		Timestamp:   time.Now().UnixMilli(),
	})
	return o, nil
}

// Reconstitute 从持久化状态重建聚合（仓储实现专用，对应 Java 的 reconstitute）。
func Reconstitute(id, customerID string, items []OrderItem, status OrderStatus, paymentID string) *Order {
	return &Order{
		id:         id,
		customerID: customerID,
		items:      items,
		status:     status,
		paymentID:  paymentID,
	}
}

// MarkAsPaid 标记订单已支付。
func (o *Order) MarkAsPaid(paymentID string) error {
	if !o.status.CanTransitionTo(StatusPaid) {
		return shared.NewBusinessRuleViolation(
			"INVALID_TRANSITION", fmt.Sprintf("Cannot pay order in status %s", o.status))
	}
	o.paymentID = paymentID
	o.status = StatusPaid
	o.RaiseEvent(event.OrderPaidEvent{
		OrderID:   o.id,
		PaymentID: paymentID,
		Timestamp: time.Now().UnixMilli(),
	})
	return nil
}

// Cancel 取消订单。
func (o *Order) Cancel(reason string) error {
	if !o.status.CanTransitionTo(StatusCancelled) {
		return shared.NewBusinessRuleViolation(
			"INVALID_TRANSITION", fmt.Sprintf("Cannot cancel order in status %s", o.status))
	}
	o.status = StatusCancelled
	o.RaiseEvent(event.OrderCancelledEvent{
		OrderID:   o.id,
		Reason:    reason,
		Timestamp: time.Now().UnixMilli(),
	})
	return nil
}

// Confirm 确认订单（支付核验 + 库存扣减后）。
func (o *Order) Confirm() error {
	if !o.status.CanTransitionTo(StatusConfirmed) {
		return shared.NewBusinessRuleViolation(
			"INVALID_TRANSITION", fmt.Sprintf("Cannot confirm order in status %s", o.status))
	}
	o.status = StatusConfirmed
	return nil
}

// TotalAmount 订单总金额。
func (o *Order) TotalAmount() decimal.Decimal {
	total := decimal.Zero
	for _, it := range o.items {
		total = total.Add(it.SubTotal())
	}
	return total
}

func (o *Order) ID() string          { return o.id }
func (o *Order) CustomerID() string  { return o.customerID }
func (o *Order) Status() OrderStatus { return o.status }
func (o *Order) PaymentID() string   { return o.paymentID }

// Items 返回订单行的防御性拷贝（对应 Java 的 Collections.unmodifiableList）。
func (o *Order) Items() []OrderItem {
	out := make([]OrderItem, len(o.items))
	copy(out, o.items)
	return out
}
