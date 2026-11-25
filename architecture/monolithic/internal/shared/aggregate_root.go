package shared

// AggregateRoot 聚合根基类 —— Go 用"结构体嵌入"模拟 Java 的 extends。
//
// 聚合根业务结构体嵌入本结构体，即可获得领域事件的收集能力：
//
//	type Order struct {
//	    shared.AggregateRoot
//	    ...
//	}
//
// 纯数据 + 方法，无任何框架依赖（对应 Java 的纯 POJO 基类）。
type AggregateRoot struct {
	domainEvents []DomainEvent
}

// RaiseEvent 记录一个领域事件（事件先暂存在聚合上，由应用层统一发布）。
func (a *AggregateRoot) RaiseEvent(event DomainEvent) {
	a.domainEvents = append(a.domainEvents, event)
}

// DomainEvents 返回事件的快照拷贝（对应 Java 的 unmodifiableList）。
func (a *AggregateRoot) DomainEvents() []DomainEvent {
	out := make([]DomainEvent, len(a.domainEvents))
	copy(out, a.domainEvents)
	return out
}

// ClearDomainEvents 清空事件（应用层发布完毕后调用，防止重复发布）。
func (a *AggregateRoot) ClearDomainEvents() {
	a.domainEvents = nil
}
