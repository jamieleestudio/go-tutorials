// 消费 SPI（对应 Java OutboxPublisher + LoggingOutboxPublisher + OrderEventHandler）。
// 捞取到事件后的处理动作由实现方决定：直接执行本地业务（更新订单状态、发通知），
// 或投递到 MQ 交给下游服务（经典本地消息表用法，可后续扩展 KafkaOutboxPublisher）。
//
// 注意：本模式是至少一次投递（at-least-once）—— 事务提交前宕机，已消费但未提交的
// 消息仍是 PENDING，会被再次捞取，因此实现方【必须保证幂等】（Java 注释原话）。
package outbox

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
)

// Publisher 消费 SPI（对应 Java OutboxPublisher）。
type Publisher interface {
	// Publish 消费一条 outbox 消息。返回错误 = 消费失败，由 relay 记录重试或转 FAILED。
	Publish(ctx context.Context, m *Message) error
}

// LoggingPublisher 默认消费实现：仅打印消息内容，用于演示与验证状态流转
// （对应 Java LoggingOutboxPublisher；业务方实现 Publisher 后在 New 里替换）。
type LoggingPublisher struct {
	logger *slog.Logger
}

// NewLoggingPublisher 建日志版消费器（logger 传 nil 用 slog 默认）。
func NewLoggingPublisher(logger *slog.Logger) *LoggingPublisher {
	if logger == nil {
		logger = slog.Default()
	}
	return &LoggingPublisher{logger: logger}
}

// Publish 打印消息即视为消费成功 —— 演示「PENDING → PUBLISHED」的主路径。
func (p *LoggingPublisher) Publish(ctx context.Context, m *Message) error {
	p.logger.Info("[LoggingPublisher] 消费事件",
		"type", m.EventType,
		"aggregate", m.AggregateType+"/"+m.AggregateID,
		"payload", m.Payload)
	return nil
}

// DemoHandler 演示用本地业务处理器（对应 Java OrderEventHandler）：
// 捞取到 ORDER_CREATED 事件后直接执行业务（打印）。
// 对聚合 ID 为 ORDER-RETRY-DEMO 的事件，前两次处理失败 —— 用于演示重试与指数退避。
type DemoHandler struct {
	logger   *slog.Logger
	failures atomic.Int32 // 对应 Java AtomicInteger retryDemoFailures
}

// NewDemoHandler 建 demo 处理器（logger 传 nil 用 slog 默认）。
func NewDemoHandler(logger *slog.Logger) *DemoHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &DemoHandler{logger: logger}
}

// Publish 前两次对 ORDER-RETRY-DEMO 抛错，第三次起成功（对应 Java 的 incrementAndGet <= 2 短路写法）。
func (h *DemoHandler) Publish(ctx context.Context, m *Message) error {
	if m.AggregateID == "ORDER-RETRY-DEMO" && h.failures.Add(1) <= 2 {
		return fmt.Errorf("模拟下游处理故障，第 %d 次失败", h.failures.Load())
	}
	h.logger.Info("[DemoHandler] 执行事件业务",
		"type", m.EventType,
		"aggregate", m.AggregateType+"/"+m.AggregateID,
		"payload", m.Payload)
	return nil
}
