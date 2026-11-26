// Package messaging 产品上下文的事件入口（对应 Java 的 OrderEventConsumer）。
//
// Java 版经 @KafkaListener 消费；Go 教学版的投递介质是 HTTP webhook：
// 上游服务的 Outbox Relay POST /internal/events。处理失败返回 5xx →
// 上游重投（at-least-once），本端靠 eventId 去重兜底。
package messaging

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/product/application"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/product/interfaces/httpmw"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared/event"
)

// OrderEventConsumer Saga 反应入口：order 事件 → 扣减 / 回补。
type OrderEventConsumer struct {
	productService application.ProductAppService
}

func NewOrderEventConsumer(productService application.ProductAppService) *OrderEventConsumer {
	return &OrderEventConsumer{productService: productService}
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
			err = c.productService.DeductForOrder(ctx.Request.Context(), envelope.EventID, &evt)
		}
	case event.TypeOrderCancelled:
		var evt event.OrderCancelledEvent
		if err = json.Unmarshal(envelope.Payload, &evt); err == nil {
			err = c.productService.RestockForOrder(ctx.Request.Context(), envelope.EventID, evt.OrderID)
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
