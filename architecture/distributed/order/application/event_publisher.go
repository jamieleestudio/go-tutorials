package application

import (
	"github.com/jamieleestudio/go-tutorials/architecture/distributed/shared"
)

// EventPublisherService 领域事件发布端口（对应 Java 的 EventPublisherService）。
//
// application 层只依赖这个端口，不感知事件总线的实现 ——
// 实现在 infrastructure（进程内 EventBus；P5 event-driven 会替换为 Outbox）。
type EventPublisherService interface {
	Publish(event shared.DomainEvent)
}
