package infrastructure

import (
	"context"
	"errors"

	"github.com/jamieleestudio/go-tutorials/architecture/microservices/payment/domain"
	"github.com/jamieleestudio/go-tutorials/architecture/microservices/shared/persistence"
	"gorm.io/gorm"
)

var _ domain.PaymentRepository = (*PaymentRepositoryImpl)(nil)

// PaymentRepositoryImpl 支付仓储实现（GORM）。
type PaymentRepositoryImpl struct{ db *gorm.DB }

func NewPaymentRepository(db *gorm.DB) *PaymentRepositoryImpl {
	return &PaymentRepositoryImpl{db: db}
}

// Save 保存支付单（upsert 语义，不用 gorm Save() 的原因见 product 仓储）。
func (r *PaymentRepositoryImpl) Save(ctx context.Context, payment *domain.Payment) error {
	db := persistence.DBFromContext(ctx, r.db)
	entity := toPaymentEntity(payment)
	res := db.Model(&PaymentEntity{}).
		Where("payment_id = ?", entity.ID).
		Updates(map[string]any{
			"order_id":       entity.OrderID,
			"amount":         entity.Amount,
			"status":         entity.Status,
			"failure_reason": entity.FailureReason,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return db.Create(entity).Error
	}
	return nil
}

func (r *PaymentRepositoryImpl) FindByID(ctx context.Context, paymentID string) (*domain.Payment, error) {
	db := persistence.DBFromContext(ctx, r.db)
	var entity PaymentEntity
	err := db.First(&entity, "payment_id = ?", paymentID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrPaymentNotFound
	}
	if err != nil {
		return nil, err
	}
	return toPaymentDomain(&entity), nil
}

// FindByOrderID 按订单查支付单 —— pay 幂等的基石。
// 找不到时返回 (nil, nil)，与 Java Optional.empty() 语义一致。
func (r *PaymentRepositoryImpl) FindByOrderID(ctx context.Context, orderID string) (*domain.Payment, error) {
	db := persistence.DBFromContext(ctx, r.db)
	var entity PaymentEntity
	err := db.First(&entity, "order_id = ?", orderID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toPaymentDomain(&entity), nil
}

func (r *PaymentRepositoryImpl) FindAll(ctx context.Context) ([]*domain.Payment, error) {
	db := persistence.DBFromContext(ctx, r.db)
	var entities []PaymentEntity
	err := db.Order("created_at").Find(&entities).Error
	if err != nil {
		return nil, err
	}
	payments := make([]*domain.Payment, 0, len(entities))
	for i := range entities {
		payments = append(payments, toPaymentDomain(&entities[i]))
	}
	return payments, nil
}
