// Package messaging 支付上下文的事件入口（对应 Java 的 OrderEventConsumer）。
//
// Java 版经 @KafkaListener 消费；Go 教学版的投递介质是 HTTP webhook：
// order 服务的 Outbox Relay POST /internal/events。处理失败返回 5xx →
// 上游重投（at-least-once），本端靠 eventId 去重兜底。
package messaging

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/payment/application"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/payment/interfaces/httpmw"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared/event"
	"github.com/shopspring/decimal"
)

// OrderEventConsumer Saga 反应入口：order 事件 → 支付 / 退款。
type OrderEventConsumer struct {
	paymentSagaService application.PaymentSagaService
}

func NewOrderEventConsumer(paymentSagaService application.PaymentSagaService) *OrderEventConsumer {
	return &OrderEventConsumer{paymentSagaService: paymentSagaService}
}

func (c *OrderEventConsumer) Register(r gin.IRouter) {
	r.POST("/internal/events", c.handle)
}

func (c *OrderEventConsumer) handle(ctx *gin.Context) {
	var envelope event.Envelope
	if !httpmw.BindJSON(ctx, &envelope) {
		return
	}

	var err error
	switch envelope.Type {
	case event.TypeOrderCreated:
		var evt event.OrderCreatedEvent
		if err = json.Unmarshal(envelope.Payload, &evt); err == nil {
			var amount decimal.Decimal
			amount, err = decimal.NewFromString(evt.TotalAmount)
			if err == nil {
				err = c.paymentSagaService.ProcessOrderPayment(ctx.Request.Context(), envelope.EventID, evt.OrderID, amount)
			}
		}
	case event.TypeOrderCancelled:
		var evt event.OrderCancelledEvent
		if err = json.Unmarshal(envelope.Payload, &evt); err == nil {
			err = c.paymentSagaService.RefundOrderPayment(ctx.Request.Context(), envelope.EventID, evt.OrderID)
		}
	default:
		// 未知类型：确认吞掉（200），避免坏消息无限重投 —— 死信话题是演进点。
		ctx.Status(http.StatusOK)
		return
	}

	if err != nil {
		// 5xx → 上游 Outbox Relay 重投 → eventId 去重兜底
		_ = ctx.Error(err)
		return
	}
	ctx.Status(http.StatusOK)
}
