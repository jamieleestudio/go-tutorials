// Package application 订单上下文的应用层。
//
// 对应 Java 的 order.application 包：定义用例接口（provider-defined interface）、
// Command / Query（CQRS 风格的入参）与 DTO（出参）。
// 实现在子包 impl 中 —— 接口层只允许依赖本包，不允许依赖 impl。
package application

import (
	"context"

	"github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/shared"
)

// CreateOrderCommand 创建订单命令（写操作）。
type CreateOrderCommand struct {
	CustomerID string
	Lines      []OrderLine
}

// OrderLine 下单行：只带产品 ID 和数量（价格由应用层查产品获得）。
type OrderLine struct {
	ProductID string
	Quantity  int
}

// PayOrderCommand 支付订单命令（写操作）。
type PayOrderCommand struct {
	OrderID string
}

// CancelOrderCommand 取消订单命令（写操作）。
type CancelOrderCommand struct {
	OrderID string
	Reason  string
}

// EventPublisher 领域事件发布端口。
// 应用层定义接口、基础设施层实现（依赖倒置），对应 Java 的 EventPublisherService。
type EventPublisher interface {
	Publish(ctx context.Context, event shared.DomainEvent) error
}
