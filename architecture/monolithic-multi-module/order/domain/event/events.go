// Package event 订单领域事件（对应 Java 的 order.domain.event 包）。
//
// 事件是纯数据结构，实现 shared.DomainEvent 接口。
// Java 的 record → Go 的普通 struct；getTimestamp() → OccurredAt()。
package event

import "github.com/shopspring/decimal"

// OrderCreatedEvent 订单创建事件。
type OrderCreatedEvent struct {
	OrderID     string
	ProductIDs  []string
	TotalAmount decimal.Decimal
	Timestamp   int64
}

func (e OrderCreatedEvent) EventName() string   { return "OrderCreatedEvent" }
func (e OrderCreatedEvent) AggregateID() string { return e.OrderID }
func (e OrderCreatedEvent) OccurredAt() int64   { return e.Timestamp }

// OrderPaidEvent 订单支付事件。
type OrderPaidEvent struct {
	OrderID   string
	PaymentID string
	Timestamp int64
}

func (e OrderPaidEvent) EventName() string   { return "OrderPaidEvent" }
func (e OrderPaidEvent) AggregateID() string { return e.OrderID }
func (e OrderPaidEvent) OccurredAt() int64   { return e.Timestamp }

// OrderCancelledEvent 订单取消事件。
type OrderCancelledEvent struct {
	OrderID   string
	Reason    string
	Timestamp int64
}

func (e OrderCancelledEvent) EventName() string   { return "OrderCancelledEvent" }
func (e OrderCancelledEvent) AggregateID() string { return e.OrderID }
func (e OrderCancelledEvent) OccurredAt() int64   { return e.Timestamp }
