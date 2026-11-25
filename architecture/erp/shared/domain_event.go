package shared

// DomainEvent 领域事件接口（对应 Java 版 erp shared.DomainEvent 标记接口）。
// Java 版是空标记接口；Go 版加一个 EventName 方法，让事件可被日志/总线识别。
type DomainEvent interface {
	EventName() string
}
