// Package mq 是 attendance 域的 mq 端（事件消费者）。
// 它实现平台层 MessageConsumer 端口，主题来自本域 api 契约（AttendanceTopics）。
package mq

import (
	"sync"

	attapi "github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/api"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/platform/messaging"
)

// 编译期断言：实现平台层消费端口。
var _ messaging.MessageConsumer = (*AttendanceClockConsumer)(nil)

// AttendanceClockConsumer 签到事件消费者（对应 Java 版 AttendanceClockConsumer）。
// 记录最近一条 payload 供演示/测试观察 —— 消费端不回写业务库，保持端职责单一。
type AttendanceClockConsumer struct {
	mu          sync.Mutex
	lastPayload string
}

// NewAttendanceClockConsumer 构造消费者。
func NewAttendanceClockConsumer() *AttendanceClockConsumer {
	return &AttendanceClockConsumer{}
}

// Topic 订阅主题（对应 Java 的 topic()）。
func (c *AttendanceClockConsumer) Topic() string { return attapi.TopicClocked }

// OnMessage 处理消息。
func (c *AttendanceClockConsumer) OnMessage(envelope messaging.MessageEnvelope) {
	c.mu.Lock()
	c.lastPayload = envelope.Payload
	c.mu.Unlock()
}

// LastPayload 最近一条消息（对应 Java 的 lastPayload()）。
func (c *AttendanceClockConsumer) LastPayload() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastPayload
}
