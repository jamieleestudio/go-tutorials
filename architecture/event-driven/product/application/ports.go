package application

import (
	"context"

	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared/event"
)

// EventPublisherService 集成事件发布端口（对应 Java 的 EventPublisherService）。
//
// 生产实现是 Outbox（事件与业务数据同事务落库），测试替身可用内存切片。
// Java 版 publish 无返回值；Go 版返回 error 让 Outbox 写失败能显式回滚事务。
type EventPublisherService interface {
	Publish(ctx context.Context, evt event.IntegrationEvent) error
}

// EventDeduplicationService 消费端幂等去重端口（对应 Java 的 EventDeduplicationService）。
//
// at-least-once 投递下重复事件必然出现：处理前查重、处理后标记，
// 查重与标记必须与业务处理在同一事务（否则进程崩溃窗口内仍可能重复处理）。
type EventDeduplicationService interface {
	IsDuplicate(ctx context.Context, eventID string) (bool, error)
	MarkProcessed(ctx context.Context, eventID string) error
}
