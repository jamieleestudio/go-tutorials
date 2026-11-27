// Package event 订单领域的三个领域事件（对应 Java 的 domain.event 包）。
// 纯数据结构，经 shared.DomainEvent 接口发布到进程内事件总线。
package event

import (
	"github.com/jamieleestudio/go-tutorials/architecture/cloud-native/shared"
	"github.com/shopspring/decimal"
)

// OrderCreatedEvent 订单创建事件。
// 负载与 Java 版一致：orderId、productIds、totalAmount、epochMillis。
type OrderCreatedEvent struct {
	OrderID     string
	ProductIDs  []string
	TotalAmount decimal.Decimal
	Timestamp   int64
}

func (e *OrderCreatedEvent) EventName() string   { return "OrderCreatedEvent" }
func (e *OrderCreatedEvent) AggregateID() string { return e.OrderID }
func (e *OrderCreatedEvent) OccurredAt() int64   { return e.Timestamp }

// OrderPaidEvent 订单支付事件。
type OrderPaidEvent struct {
	OrderID   string
	PaymentID string
	Timestamp int64
}

func (e *OrderPaidEvent) EventName() string   { return "OrderPaidEvent" }
func (e *OrderPaidEvent) AggregateID() string { return e.OrderID }
func (e *OrderPaidEvent) OccurredAt() int64   { return e.Timestamp }

// OrderCancelledEvent 订单取消事件。
type OrderCancelledEvent struct {
	OrderID   string
	Reason    string
	Timestamp int64
}

func (e *OrderCancelledEvent) EventName() string   { return "OrderCancelledEvent" }
func (e *OrderCancelledEvent) AggregateID() string { return e.OrderID }
func (e *OrderCancelledEvent) OccurredAt() int64   { return e.Timestamp }

// 编译期断言：事件实现 shared.DomainEvent。
var (
	_ shared.DomainEvent = (*OrderCreatedEvent)(nil)
	_ shared.DomainEvent = (*OrderPaidEvent)(nil)
	_ shared.DomainEvent = (*OrderCancelledEvent)(nil)
)
