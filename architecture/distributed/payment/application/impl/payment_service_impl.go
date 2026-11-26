// Package impl PaymentAppService 的实现（对应 Java 的 PaymentServiceImpl）。
package impl

import (
	"context"

	"github.com/jamieleestudio/go-tutorials/architecture/distributed/payment/api"
	"github.com/jamieleestudio/go-tutorials/architecture/distributed/payment/application"
	"github.com/jamieleestudio/go-tutorials/architecture/distributed/payment/domain"
	"github.com/jamieleestudio/go-tutorials/architecture/distributed/shared"
	"github.com/shopspring/decimal"
)

var _ application.PaymentAppService = (*PaymentServiceImpl)(nil)

// PaymentServiceImpl 应用服务实现。
type PaymentServiceImpl struct {
	paymentRepository domain.PaymentRepository
}

func NewPaymentServiceImpl(paymentRepository domain.PaymentRepository) *PaymentServiceImpl {
	return &PaymentServiceImpl{paymentRepository: paymentRepository}
}

func (s *PaymentServiceImpl) CreatePayment(ctx context.Context, cmd application.CreatePaymentCommand) (*api.PaymentDto, error) {
	payment, err := domain.Create(cmd.OrderID, cmd.Amount)
	if err != nil {
		return nil, err
	}
	if err := s.paymentRepository.Save(ctx, payment); err != nil {
		return nil, err
	}
	return toDto(payment), nil
}

func (s *PaymentServiceImpl) ProcessPayment(ctx context.Context, cmd application.ProcessPaymentCommand) (*api.PaymentDto, error) {
	payment, err := s.findPaymentOrThrow(ctx, cmd.PaymentID)
	if err != nil {
		return nil, err
	}
	// Simulate payment processing — always succeeds in demo
	if err := payment.Succeed(); err != nil {
		return nil, err
	}
	if err := s.paymentRepository.Save(ctx, payment); err != nil {
		return nil, err
	}
	return toDto(payment), nil
}

func (s *PaymentServiceImpl) GetPaymentByID(ctx context.Context, query application.GetPaymentByIDQuery) (*api.PaymentDto, error) {
	payment, err := s.findPaymentOrThrow(ctx, query.PaymentID)
	if err != nil {
		return nil, err
	}
	return toDto(payment), nil
}

func (s *PaymentServiceImpl) GetAllPayments(ctx context.Context, _ application.GetAllPaymentsQuery) ([]*api.PaymentDto, error) {
	payments, err := s.paymentRepository.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	dtos := make([]*api.PaymentDto, 0, len(payments))
	for _, p := range payments {
		dtos = append(dtos, toDto(p))
	}
	return dtos, nil
}

// Pay 跨服务契约方法（gRPC 暴露给 order 服务）。
//
// 分布式幂等：RPC 重试不能产生重复支付。同一订单 → 同一支付单 ID
// （orderId 查重 + 状态检查，对应 Java 版同语义）。
func (s *PaymentServiceImpl) Pay(ctx context.Context, orderID string, amount decimal.Decimal) (string, error) {
	existing, err := s.paymentRepository.FindByOrderID(ctx, orderID)
	if err != nil {
		return "", err
	}
	if existing != nil {
		return existing.ID(), nil
	}
	payment, err := domain.Create(orderID, amount)
	if err != nil {
		return "", err
	}
	// Simulate successful payment
	if err := payment.Succeed(); err != nil {
		return "", err
	}
	if err := s.paymentRepository.Save(ctx, payment); err != nil {
		return "", err
	}
	return payment.ID(), nil
}

// Refund 跨服务契约方法。
func (s *PaymentServiceImpl) Refund(ctx context.Context, paymentID string) error {
	payment, err := s.findPaymentOrThrow(ctx, paymentID)
	if err != nil {
		return err
	}
	if err := payment.Refund(); err != nil {
		return err
	}
	return s.paymentRepository.Save(ctx, payment)
}

func (s *PaymentServiceImpl) findPaymentOrThrow(ctx context.Context, paymentID string) (*domain.Payment, error) {
	payment, err := s.paymentRepository.FindByID(ctx, paymentID)
	if err != nil {
		if err == domain.ErrPaymentNotFound {
			return nil, shared.NewEntityNotFound("PAYMENT_NOT_FOUND", "Payment not found: "+paymentID)
		}
		return nil, err
	}
	return payment, nil
}

func toDto(payment *domain.Payment) *api.PaymentDto {
	return &api.PaymentDto{
		PaymentID: payment.ID(),
		OrderID:   payment.OrderID(),
		Amount:    payment.Amount(),
		Status:    string(payment.Status()),
	}
}
