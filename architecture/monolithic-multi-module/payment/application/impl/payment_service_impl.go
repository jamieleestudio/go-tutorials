// Package impl 支付应用服务实现（对应 Java 的 PaymentServiceImpl）。
package impl

import (
	"context"
	"errors"

	paymentapp "github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/payment/application"
	paymentdomain "github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/payment/domain"
	"github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/shared"
	"github.com/shopspring/decimal"
)

var _ paymentapp.PaymentService = (*PaymentServiceImpl)(nil)

// PaymentServiceImpl 支付应用服务实现。
type PaymentServiceImpl struct {
	paymentRepo paymentdomain.PaymentRepository
	txRunner    shared.TxRunner
}

func NewPaymentService(paymentRepo paymentdomain.PaymentRepository, txRunner shared.TxRunner) *PaymentServiceImpl {
	return &PaymentServiceImpl{paymentRepo: paymentRepo, txRunner: txRunner}
}

func (s *PaymentServiceImpl) CreatePayment(ctx context.Context, cmd paymentapp.CreatePaymentCommand) (*paymentapp.PaymentDto, error) {
	var dto *paymentapp.PaymentDto
	err := s.txRunner.WithinTx(ctx, func(ctx context.Context) error {
		payment, err := paymentdomain.Create(cmd.OrderID, cmd.Amount)
		if err != nil {
			return err
		}
		if err := s.paymentRepo.Save(ctx, payment); err != nil {
			return err
		}
		dto = toPaymentDto(payment)
		return nil
	})
	return dto, err
}

// ProcessPayment 处理支付 —— demo 中模拟为必然成功。
func (s *PaymentServiceImpl) ProcessPayment(ctx context.Context, cmd paymentapp.ProcessPaymentCommand) (*paymentapp.PaymentDto, error) {
	var dto *paymentapp.PaymentDto
	err := s.txRunner.WithinTx(ctx, func(ctx context.Context) error {
		payment, err := s.findPayment(ctx, cmd.PaymentID)
		if err != nil {
			return err
		}
		if err := payment.Succeed(); err != nil {
			return err
		}
		if err := s.paymentRepo.Save(ctx, payment); err != nil {
			return err
		}
		dto = toPaymentDto(payment)
		return nil
	})
	return dto, err
}

func (s *PaymentServiceImpl) GetPaymentByID(ctx context.Context, query paymentapp.GetPaymentByIDQuery) (*paymentapp.PaymentDto, error) {
	payment, err := s.findPayment(ctx, query.PaymentID)
	if err != nil {
		return nil, err
	}
	return toPaymentDto(payment), nil
}

func (s *PaymentServiceImpl) GetAllPayments(ctx context.Context, _ paymentapp.GetAllPaymentsQuery) ([]paymentapp.PaymentDto, error) {
	payments, err := s.paymentRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	dtos := make([]paymentapp.PaymentDto, 0, len(payments))
	for _, p := range payments {
		dtos = append(dtos, *toPaymentDto(p))
	}
	return dtos, nil
}

// Pay 跨上下文入口：创建支付单并立即成功，返回支付单 ID。
func (s *PaymentServiceImpl) Pay(ctx context.Context, orderID string, amount decimal.Decimal) (string, error) {
	var paymentID string
	err := s.txRunner.WithinTx(ctx, func(ctx context.Context) error {
		payment, err := paymentdomain.Create(orderID, amount)
		if err != nil {
			return err
		}
		// 模拟支付成功
		if err := payment.Succeed(); err != nil {
			return err
		}
		if err := s.paymentRepo.Save(ctx, payment); err != nil {
			return err
		}
		paymentID = payment.ID()
		return nil
	})
	return paymentID, err
}

func (s *PaymentServiceImpl) Refund(ctx context.Context, paymentID string) error {
	return s.txRunner.WithinTx(ctx, func(ctx context.Context) error {
		payment, err := s.findPayment(ctx, paymentID)
		if err != nil {
			return err
		}
		if err := payment.Refund(); err != nil {
			return err
		}
		return s.paymentRepo.Save(ctx, payment)
	})
}

func (s *PaymentServiceImpl) findPayment(ctx context.Context, paymentID string) (*paymentdomain.Payment, error) {
	payment, err := s.paymentRepo.FindByID(ctx, paymentID)
	if err != nil {
		if errors.Is(err, paymentdomain.ErrPaymentNotFound) {
			return nil, shared.NewEntityNotFound("PAYMENT_NOT_FOUND", "Payment not found: "+paymentID)
		}
		return nil, err
	}
	return payment, nil
}

func toPaymentDto(p *paymentdomain.Payment) *paymentapp.PaymentDto {
	return &paymentapp.PaymentDto{
		PaymentID: p.ID(),
		OrderID:   p.OrderID(),
		Amount:    p.Amount(),
		Status:    string(p.Status()),
	}
}
