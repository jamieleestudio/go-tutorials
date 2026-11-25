package infrastructure

import (
	"context"

	orderapp "github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/order/application"
	"github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/shared"
)

// 编译期断言：实现应用层定义的事件发布端口（依赖倒置）。
var _ orderapp.EventPublisher = (*EventPublisherImpl)(nil)

// EventPublisherImpl 基于 shared.EventBus 发布领域事件。
// 对应 Java 的 SpringEventPublisherServiceImpl + ApplicationEventPublisher。
type EventPublisherImpl struct {
	bus *shared.EventBus
}

func NewEventPublisher(bus *shared.EventBus) *EventPublisherImpl {
	return &EventPublisherImpl{bus: bus}
}

// Publish 同步发布事件。ctx 参数保留在签名里：
// P5（event-driven）升级为 Outbox 异步投递时会真正用到它。
func (p *EventPublisherImpl) Publish(_ context.Context, event shared.DomainEvent) error {
	p.bus.Publish(event)
	return nil
}
