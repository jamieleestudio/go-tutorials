// Package event 订单事件监听器（对应 Java 的 order.interfaces.event.OrderEventListener）。
//
// Java 用 @EventListener 注解 + 反射分发；Go 显式按事件名订阅，
// 处理时用类型断言还原具体事件 —— 一切都是明式代码。
package event

import (
	"log/slog"

	orderevent "github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/order/domain/event"
	"github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/shared"
)

// RegisterHandlers 把订单事件监听器注册到进程内事件总线。
func RegisterHandlers(bus *shared.EventBus) {
	bus.Subscribe("OrderCreatedEvent", func(e shared.DomainEvent) {
		ev, ok := e.(orderevent.OrderCreatedEvent)
		if !ok {
			return
		}
		slog.Info("Order created",
			"orderId", ev.OrderID,
			"productIds", ev.ProductIDs,
			"total", ev.TotalAmount)
	})

	bus.Subscribe("OrderPaidEvent", func(e shared.DomainEvent) {
		ev, ok := e.(orderevent.OrderPaidEvent)
		if !ok {
			return
		}
		slog.Info("Order paid", "orderId", ev.OrderID, "paymentId", ev.PaymentID)
	})

	bus.Subscribe("OrderCancelledEvent", func(e shared.DomainEvent) {
		ev, ok := e.(orderevent.OrderCancelledEvent)
		if !ok {
			return
		}
		slog.Info("Order cancelled", "orderId", ev.OrderID, "reason", ev.Reason)
	})
}
