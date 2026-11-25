package shared

// AggregateRoot 聚合根基类（对应 Java 版 erp shared.AggregateRoot）。
// 业务聚合嵌入它获得领域事件的收集能力。
type AggregateRoot struct {
	domainEvents []DomainEvent
}

// RegisterEvent 记录领域事件（Java 版的方法名，保留以利对照）。
func (a *AggregateRoot) RegisterEvent(event DomainEvent) {
	a.domainEvents = append(a.domainEvents, event)
}

// DomainEvents 返回事件快照。
func (a *AggregateRoot) DomainEvents() []DomainEvent {
	out := make([]DomainEvent, len(a.domainEvents))
	copy(out, a.domainEvents)
	return out
}

// ClearDomainEvents 清空事件。
func (a *AggregateRoot) ClearDomainEvents() {
	a.domainEvents = nil
}
