package shared

// DomainEvent 领域事件接口（对应 Java 的 shared.DomainEvent 标记接口）。
//
// 领域事件表示"领域中发生的一件有业务意义的事"，是纯数据结构。
type DomainEvent interface {
	// EventName 事件类型名，作为进程内事件总线的 topic。
	// （Java 版靠事件类型做 @EventListener 匹配；Go 用显式的事件名。）
	EventName() string
	// AggregateID 产生该事件的聚合 ID（对应 getAggregateId）。
	AggregateID() string
	// OccurredAt 事件发生时间（epoch 毫秒），对应 Java 的 getTimestamp()。
	OccurredAt() int64
}
