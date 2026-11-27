// Package outbox 对应 java-tutorials/distributed-infra/distributed-solutions/distributed-solutions-outbox
// —— Outbox 本地消息表的【完整语义版】（对照 architecture/event-driven 的 P5 简化版）：
//
//	MySQL 残留：唯一索引锁崩溃后行残留、锁永久卡死 —— 无过期，需人工清理
//	Redis 过期：TTL 到期自愈，但长任务可能丢锁（无看门狗续期）
//	ZK 会话：  临时节点会话断开自动清理 —— 最优雅，但性能最低
//
// 差异对照（为什么 P5 不能替代本包）：
//
//	P5 简化版（architecture/event-driven）：SQLite 内存库 + FindPending 无认领语义
//	            —— 单进程教学可用，多实例并发会重复消费
//	本包完整版：真 MySQL + FOR UPDATE SKIP LOCKED 认领（多实例不重复不阻塞）+
//	            指数退避重试 + FAILED 死信 + at-least-once 幂等要求
//
// 逐文件对应 Java 版：
//
//	message.go    ← OutboxMessage + OutboxStatus（实体 + 状态机 + Pending 工厂）
//	schema.go     ← schema.sql + DemoOrder/DemoOrderRepository（DDL + 业务表 + 仓库）
//	publisher.go  ← OutboxPublisher SPI + LoggingOutboxPublisher + OrderEventHandler
//	relay.go      ← OutboxRelayProperties + OutboxRelay（认领 SQL + relayOnce + 退避）
//	demo/         ← OutboxDemoApplication + OutboxDemoRunner（三场景演示）
package outbox

import "time"

// Status 消息状态机：PENDING → PUBLISHED（成功）| FAILED（重试超限死信）。
// 对应 Java 枚举 OutboxStatus。FAILED 不是终局重投 —— 是死信，需人工介入或对账补偿。
type Status string

const (
	StatusPending   Status = "PENDING"   // 待消费（含等待退避窗口）
	StatusPublished Status = "PUBLISHED" // 已消费
	StatusFailed    Status = "FAILED"    // 重试超限，转死信
)

// Message 本地消息表实体（对应 Java OutboxMessage，字段一一对应）。
type Message struct {
	ID            int64
	AggregateType string // 聚合类型，如 ORDER —— 事件归属的业务域
	AggregateID   string // 聚合业务 ID，如订单号 —— 便于问题追溯
	EventType     string // 事件类型，如 ORDER_CREATED
	Payload       string // 事件内容（JSON），消费方据此执行业务
	Status        Status
	RetryCount    int       // 已失败重试次数
	NextRetryAt   time.Time // 到期才可被定时任务捞取（退避机制）
	CreatedAt     time.Time
	PublishedAt   *time.Time // 消费成功时间（nil = 未消费）
}

// Pending 工厂（对应 Java OutboxMessage.pending）：状态 PENDING、重试 0 次、
// nextRetryAt = createdAt —— 落库即到期，下一轮 relay 就能捞走。
func Pending(aggregateType, aggregateID, eventType, payload string) *Message {
	now := time.Now().UTC()
	return &Message{
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       payload,
		Status:        StatusPending,
		NextRetryAt:   now,
		CreatedAt:     now,
	}
}
