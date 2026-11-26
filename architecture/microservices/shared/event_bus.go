package shared

import "sync"

// EventBus 进程内同步事件总线。
//
// 对应 Java 版的 Spring ApplicationEventPublisher：
//   - Publish 按订阅顺序【同步】调用处理器（与 @EventListener 默认行为一致）；
//   - 订阅者通过事件名（EventName）匹配，处理时用类型断言还原具体事件。
//
// P5（event-driven）会把"同步总线"升级为"Outbox + 异步投递"。
type EventBus struct {
	mu       sync.RWMutex
	handlers map[string][]func(DomainEvent)
}

func NewEventBus() *EventBus {
	return &EventBus{handlers: make(map[string][]func(DomainEvent))}
}

// Subscribe 按事件名注册处理器（对应 @EventListener）。
func (b *EventBus) Subscribe(eventName string, handler func(DomainEvent)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventName] = append(b.handlers[eventName], handler)
}

// Publish 同步发布事件。
func (b *EventBus) Publish(event DomainEvent) {
	b.mu.RLock()
	handlers := b.handlers[event.EventName()]
	b.mu.RUnlock()
	for _, h := range handlers {
		h(event)
	}
}
