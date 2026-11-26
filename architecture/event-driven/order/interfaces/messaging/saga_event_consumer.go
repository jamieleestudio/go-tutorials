// Package messaging 订单上下文的事件入口（对应 Java 的 PaymentEventConsumer +
// InventoryEventConsumer —— Java 两个 @KafkaListener，Go 教学版一个 webhook
// 端点收两个 topic：payment-events 与 product-events 的投递目标都是这里）。
package messaging

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/order/application"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/order/interfaces/httpmw"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared/event"
)

// SagaEventConsumer Saga 反应入口：payment / inventory 事件 → 订单确认或取消。
type SagaEventConsumer struct {
	orderService application.OrderService
}

func NewSagaEventConsumer(orderService application.OrderService) *SagaEventConsumer {
	return &SagaEventConsumer{orderService: orderService}
}

func (c *SagaEventConsumer) Register(r gin.IRouter) {
	r.POST("/internal/events", c.handle)
}

func (c *SagaEventConsumer) handle(ctx *gin.Context) {
	var envelope event.Envelope
	if !httpmw.BindJSON(ctx, &envelope) {
		return
	}

	var err error
	switch envelope.Type {
	case event.TypePaymentSucceeded:
		var evt event.PaymentSucceededEvent
		if err = json.Unmarshal(envelope.Payload, &evt); err == nil {
			err = c.orderService.OnPaymentSucceeded(ctx.Request.Context(), envelope.EventID, evt.OrderID, evt.PaymentID)
		}
	case event.TypePaymentFailed:
		var evt event.PaymentFailedEvent
		if err = json.Unmarshal(envelope.Payload, &evt); err == nil {
			err = c.orderService.OnPaymentFailed(ctx.Request.Context(), envelope.EventID, evt.OrderID, evt.Reason)
		}
	case event.TypeInventoryDeducted:
		var evt event.InventoryDeductedEvent
		if err = json.Unmarshal(envelope.Payload, &evt); err == nil {
			err = c.orderService.OnInventoryDeducted(ctx.Request.Context(), envelope.EventID, evt.OrderID)
		}
	case event.TypeInventoryDeductFailed:
		var evt event.InventoryDeductFailedEvent
		if err = json.Unmarshal(envelope.Payload, &evt); err == nil {
			err = c.orderService.OnInventoryDeductFailed(ctx.Request.Context(), envelope.EventID, evt.OrderID, evt.Reason)
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
