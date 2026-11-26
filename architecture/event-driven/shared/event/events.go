// Package event 集成事件契约（对应 Java eda-shared-kernel 的 shared.event 包）。
//
// 与领域事件（shared.DomainEvent，上下文内部）相对，集成事件跨越上下文边界：
// 由应用层经 EventPublisherService 端口发布（Outbox 落库），经 Relay 异步投递。
//
// Kafka 对应物：Topic() 即 Kafka topic，AggregateID() 即 Kafka message key
// （同聚合的事件保序）。P5 教学版的投递介质是 HTTP webhook（零外部设施，
// at-least-once 语义不变）；Kafka/NATS 是接缝处的演进点（见 README）。
package event

import (
	"encoding/json"

	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared"
)

// topic 常量（对应 Java 版各事件的 topic() 返回值）。
const (
	TopicOrderEvents   = "order-events"
	TopicPaymentEvents = "payment-events"
	TopicProductEvents = "product-events"
)

// IntegrationEvent 集成事件契约：跨上下文发布到消息总线的事件。
type IntegrationEvent interface {
	// Topic 事件发布到的 topic。
	Topic() string
	// AggregateID 聚合 ID —— 作为消息 key（同聚合事件保序）。
	AggregateID() string
	// TypeName 事件类型名（对应 Java 的 getClass().getSimpleName()），
	// 消费端按它 switch 分发。
	TypeName() string
}

// Envelope 在总线上旅行的信封。
// eventId 驱动消费端幂等（at-least-once 投递下重复到达必须能被识别并跳过）。
type Envelope struct {
	EventID     string          `json:"eventId"`
	Type        string          `json:"type"`
	AggregateID string          `json:"aggregateId"`
	Topic       string          `json:"topic"`
	Payload     json.RawMessage `json:"payload"`
}

// Encode 集成事件 → Envelope（生成 eventId，序列化 payload）。
func Encode(evt IntegrationEvent) (Envelope, error) {
	payload, err := json.Marshal(evt)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{
		EventID:     shared.NewID(),
		Type:        evt.TypeName(),
		AggregateID: evt.AggregateID(),
		Topic:       evt.Topic(),
		Payload:     payload,
	}, nil
}

// 事件类型名常量（消费端 switch 分发用，对应 Java consumer 里的字符串字面量）。
const (
	TypeOrderCreated          = "OrderCreatedEvent"
	TypeOrderCancelled        = "OrderCancelledEvent"
	TypePaymentSucceeded      = "PaymentSucceededEvent"
	TypePaymentFailed         = "PaymentFailedEvent"
	TypeInventoryDeducted     = "InventoryDeductedEvent"
	TypeInventoryDeductFailed = "InventoryDeductFailedEvent"
)

// ---- 六个集成事件（对应 Java 版的 record）----
// 金额以 string 承载 decimal（与 P4 的 protobuf 契约一致）。

// OrderCreatedEvent order 服务创建订单后发布 —— 启动 Saga（支付 + 扣库存）。
type OrderCreatedEvent struct {
	OrderID     string      `json:"orderId"`
	CustomerID  string      `json:"customerId"`
	Lines       []OrderLine `json:"lines"`
	TotalAmount string      `json:"totalAmount"`
}

type OrderLine struct {
	ProductID   string `json:"productId"`
	ProductName string `json:"productName"`
	Quantity    int    `json:"quantity"`
	UnitPrice   string `json:"unitPrice"`
}

func (e *OrderCreatedEvent) Topic() string       { return TopicOrderEvents }
func (e *OrderCreatedEvent) AggregateID() string { return e.OrderID }
func (e *OrderCreatedEvent) TypeName() string    { return TypeOrderCreated }

// OrderCancelledEvent order 服务取消订单后发布 —— 触发补偿（退款 + 回补库存）。
type OrderCancelledEvent struct {
	OrderID string `json:"orderId"`
	Reason  string `json:"reason"`
}

func (e *OrderCancelledEvent) Topic() string       { return TopicOrderEvents }
func (e *OrderCancelledEvent) AggregateID() string { return e.OrderID }
func (e *OrderCancelledEvent) TypeName() string    { return TypeOrderCancelled }

// PaymentSucceededEvent payment 服务支付成功后发布。
type PaymentSucceededEvent struct {
	OrderID   string `json:"orderId"`
	PaymentID string `json:"paymentId"`
}

func (e *PaymentSucceededEvent) Topic() string       { return TopicPaymentEvents }
func (e *PaymentSucceededEvent) AggregateID() string { return e.OrderID }
func (e *PaymentSucceededEvent) TypeName() string    { return TypePaymentSucceeded }

// PaymentFailedEvent payment 服务支付失败后发布 —— 触发订单取消补偿。
type PaymentFailedEvent struct {
	OrderID string `json:"orderId"`
	Reason  string `json:"reason"`
}

func (e *PaymentFailedEvent) Topic() string       { return TopicPaymentEvents }
func (e *PaymentFailedEvent) AggregateID() string { return e.OrderID }
func (e *PaymentFailedEvent) TypeName() string    { return TypePaymentFailed }

// InventoryDeductedEvent product 服务扣减成功后发布。
type InventoryDeductedEvent struct {
	OrderID string `json:"orderId"`
}

func (e *InventoryDeductedEvent) Topic() string       { return TopicProductEvents }
func (e *InventoryDeductedEvent) AggregateID() string { return e.OrderID }
func (e *InventoryDeductedEvent) TypeName() string    { return TypeInventoryDeducted }

// InventoryDeductFailedEvent product 服务扣减失败后发布（如缺货）—— 触发订单取消补偿。
type InventoryDeductFailedEvent struct {
	OrderID string `json:"orderId"`
	Reason  string `json:"reason"`
}

func (e *InventoryDeductFailedEvent) Topic() string       { return TopicProductEvents }
func (e *InventoryDeductFailedEvent) AggregateID() string { return e.OrderID }
func (e *InventoryDeductFailedEvent) TypeName() string    { return TypeInventoryDeductFailed }

// 编译期断言：全部实现 IntegrationEvent。
var (
	_ IntegrationEvent = (*OrderCreatedEvent)(nil)
	_ IntegrationEvent = (*OrderCancelledEvent)(nil)
	_ IntegrationEvent = (*PaymentSucceededEvent)(nil)
	_ IntegrationEvent = (*PaymentFailedEvent)(nil)
	_ IntegrationEvent = (*InventoryDeductedEvent)(nil)
	_ IntegrationEvent = (*InventoryDeductFailedEvent)(nil)
)
