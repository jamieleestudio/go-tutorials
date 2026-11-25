// Package messaging 是平台层的消息底座（对应 Java 版 erp-platform-messaging）。
//
// MessagePublisher/MessageConsumer 是端口，InMemoryMessagePublisher 是演示实现：
// publish 时按 topic 同步分发给所有匹配的 consumer（进程内事件流）。
// P4 引入 gRPC 后可替换为真正的 MQ 实现，业务代码不动。
package messaging

import "time"

// MessageEnvelope 消息信封（对应 Java 版 MessageEnvelope record）。
type MessageEnvelope struct {
	Topic      string    `json:"topic"`
	Payload    string    `json:"payload"`
	OccurredAt time.Time `json:"occurredAt"`
}

// Of 构造信封（对应 Java 的 MessageEnvelope.of）。
func Of(topic, payload string) MessageEnvelope {
	return MessageEnvelope{Topic: topic, Payload: payload, OccurredAt: time.Now()}
}

// MessagePublisher 消息发布端口。
type MessagePublisher interface {
	Publish(envelope MessageEnvelope)
}

// MessageConsumer 消息消费端口（对应 Java 版 MessageConsumer 接口）。
type MessageConsumer interface {
	Topic() string
	OnMessage(envelope MessageEnvelope)
}

// InMemoryMessagePublisher 进程内消息分发（对应 Java 版 InMemoryMessagePublisher）。
//
// Java 版通过构造器注入 List<MessageConsumer> 自动收集全部消费者；
// Go 没有 classpath 扫描，由组合根（app/assemble）显式收集传入。
type InMemoryMessagePublisher struct {
	consumers []MessageConsumer
}

// NewInMemoryMessagePublisher 构造，consumers 由组合根收集。
func NewInMemoryMessagePublisher(consumers []MessageConsumer) *InMemoryMessagePublisher {
	return &InMemoryMessagePublisher{consumers: consumers}
}

// Publish 按 topic 分发给全部匹配的 consumer（同步，进程内）。
func (p *InMemoryMessagePublisher) Publish(envelope MessageEnvelope) {
	for _, consumer := range p.consumers {
		if consumer.Topic() == envelope.Topic {
			consumer.OnMessage(envelope)
		}
	}
}
