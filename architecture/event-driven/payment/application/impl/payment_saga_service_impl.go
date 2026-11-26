// Package impl PaymentSagaService 的实现（对应 Java 的 PaymentSagaServiceImpl）。
package impl

import (
	"context"
	"log/slog"

	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/payment/application"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/payment/domain"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared/event"
	"github.com/shopspring/decimal"
)

var _ application.PaymentSagaService = (*PaymentSagaServiceImpl)(nil)

// PaymentSagaServiceImpl Saga 支付侧实现。
type PaymentSagaServiceImpl struct {
	paymentRepository domain.PaymentRepository
	txRunner          shared.TxRunner
	eventPublisher    application.EventPublisherService
	eventDedup        application.EventDeduplicationService
}

func NewPaymentSagaServiceImpl(
	paymentRepository domain.PaymentRepository,
	txRunner shared.TxRunner,
	eventPublisher application.EventPublisherService,
	eventDedup application.EventDeduplicationService,
) *PaymentSagaServiceImpl {
	return &PaymentSagaServiceImpl{
		paymentRepository: paymentRepository,
		txRunner:          txRunner,
		eventPublisher:    eventPublisher,
		eventDedup:        eventDedup,
	}
}

// ProcessOrderPayment OrderCreatedEvent → 支付 → PaymentSucceeded。
//
// 幂等双保险：
//  1. eventId 去重（processed_events 表）—— 重复投递直接跳过；
//  2. findByOrderId 业务幂等 —— 不同 eventId 携带同一订单（如 relay 全量重放）
//     也不会创建第二笔支付。
//
// 支付 + PaymentSucceeded 落 outbox 在同一本地事务（Outbox 模式核心）。
// 与 Java 一致：业务规则失败不 catch（金额非法等契约破坏会回滚并重投）。
func (s *PaymentSagaServiceImpl) ProcessOrderPayment(ctx context.Context, eventID string, orderID string, amount decimal.Decimal) error {
	return s.txRunner.WithinTx(ctx, func(ctx context.Context) error {
		duplicate, err := s.eventDedup.IsDuplicate(ctx, eventID)
		if err != nil {
			return err
		}
		if duplicate {
			slog.Info("duplicate OrderCreatedEvent, skipping", "eventId", eventID, "orderId", orderID)
			return nil
		}

		// 业务幂等：同一订单已有支付 → 标记后返回（不重复支付、不重复发布）
		existing, err := s.paymentRepository.FindByOrderID(ctx, orderID)
		if err != nil {
			return err
		}
		if existing != nil {
			slog.Info("payment already exists for order, skipping", "orderId", orderID, "paymentId", existing.ID())
			return s.eventDedup.MarkProcessed(ctx, eventID)
		}

		payment, err := domain.Create(orderID, amount)
		if err != nil {
			return err
		}
		if err := payment.Succeed(); err != nil { // demo：模拟支付网关，处理即成功
			return err
		}
		if err := s.paymentRepository.Save(ctx, payment); err != nil {
			return err
		}

		// 同一本地事务：支付落库 + outbox 落库
		if err := s.eventPublisher.Publish(ctx, &event.PaymentSucceededEvent{
			OrderID:   orderID,
			PaymentID: payment.ID(),
		}); err != nil {
			return err
		}
		if err := s.eventDedup.MarkProcessed(ctx, eventID); err != nil {
			return err
		}
		slog.Info("payment succeeded", "orderId", orderID, "paymentId", payment.ID())
		return nil
	})
}

// RefundOrderPayment OrderCancelledEvent → 退款（Saga 补偿）。
// 没有支付单则跳过；非 SUCCESS 状态的退款被领域守卫拒绝
// （回滚 + 重投 —— 与 Java 版抛 BusinessRuleViolation 等价）。
func (s *PaymentSagaServiceImpl) RefundOrderPayment(ctx context.Context, eventID string, orderID string) error {
	return s.txRunner.WithinTx(ctx, func(ctx context.Context) error {
		duplicate, err := s.eventDedup.IsDuplicate(ctx, eventID)
		if err != nil {
			return err
		}
		if duplicate {
			slog.Info("duplicate OrderCancelledEvent, skipping", "eventId", eventID, "orderId", orderID)
			return nil
		}

		payment, err := s.paymentRepository.FindByOrderID(ctx, orderID)
		if err != nil {
			return err
		}
		if payment != nil {
			if err := payment.Refund(); err != nil {
				return err
			}
			if err := s.paymentRepository.Save(ctx, payment); err != nil {
				return err
			}
			slog.Info("refunded payment for cancelled order", "orderId", orderID, "paymentId", payment.ID())
		}
		return s.eventDedup.MarkProcessed(ctx, eventID)
	})
}
