package application

import (
	"context"

	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared/event"
)

// EventPublisherService 集成事件发布端口（对应 Java 的 EventPublisherService）。
//
// 生产实现是 Outbox（事件与业务数据同事务落库），测试替身可用内存切片。
type EventPublisherService interface {
	Publish(ctx context.Context, evt event.IntegrationEvent) error
}

// EventDeduplicationService 消费端幂等去重端口（对应 Java 的 EventDeduplicationService）。
type EventDeduplicationService interface {
	IsDuplicate(ctx context.Context, eventID string) (bool, error)
	MarkProcessed(ctx context.Context, eventID string) error
}
