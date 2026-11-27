// Package event 订单基础设施的事件实现。
package event

import (
	"log/slog"

	domainevent "github.com/jamieleestudio/go-tutorials/architecture/cloud-native/order/domain/event"
	"github.com/jamieleestudio/go-tutorials/architecture/cloud-native/shared"
)

// EventPublisherImpl application.EventPublisherService 端口的进程内实现。
//
// 对应 Java 版 Spring 的 ApplicationEventPublisher：同步发布到进程内总线。
// P5（event-driven）会把这里升级为 Outbox + 消息代理投递。
type EventPublisherImpl struct {
	bus *shared.EventBus
}

func NewEventPublisher(bus *shared.EventBus) *EventPublisherImpl {
	return &EventPublisherImpl{bus: bus}
}

func (p *EventPublisherImpl) Publish(event shared.DomainEvent) {
	p.bus.Publish(event)
}

// OrderEventListener 事件监听器（对应 Java 的 OrderEventListener，interfaces 层）。
// 只订阅 OrderCreatedEvent 打日志 —— 与 Java 版保持一致。
type OrderEventListener struct{}

func NewOrderEventListener() *OrderEventListener {
	return &OrderEventListener{}
}

// Register 把监听器挂到总线上（对应 @EventListener 方法注册）。
func (l *OrderEventListener) Register(bus *shared.EventBus) {
	bus.Subscribe("OrderCreatedEvent", func(e shared.DomainEvent) {
		ev, ok := e.(*domainevent.OrderCreatedEvent)
		if !ok {
			return
		}
		slog.Info("Order created",
			"orderId", ev.OrderID,
			"productIds", ev.ProductIDs,
			"total", ev.TotalAmount.StringFixed(2))
	})
}
