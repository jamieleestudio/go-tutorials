package infrastructure

import (
	"context"
	"errors"

	"github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/platform/persistence"
	paymentdomain "github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/payment/domain"
	"gorm.io/gorm"
)

var _ paymentdomain.PaymentRepository = (*PaymentRepositoryImpl)(nil)

// PaymentRepositoryImpl 支付仓储实现（GORM）。
type PaymentRepositoryImpl struct{ db *gorm.DB }

func NewPaymentRepository(db *gorm.DB) *PaymentRepositoryImpl {
	return &PaymentRepositoryImpl{db: db}
}

// Save 保存支付单（upsert 语义）：
//   已存在 → 按列名显式更新（不碰 created_at，零值字段也会写入）
//   不存在 → 插入（GORM 自动填充 created_at / updated_at）
// 不用 gorm 的 Save() 做 upsert 的原因见 order_repository_impl.go。
func (r *PaymentRepositoryImpl) Save(ctx context.Context, payment *paymentdomain.Payment) error {
	db := persistence.DBFromContext(ctx, r.db)
	entity := toPaymentEntity(payment)
	res := db.Model(&PaymentEntity{}).
		Where("payment_id = ?", entity.ID).
		Updates(map[string]any{
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

func (r *PaymentRepositoryImpl) FindByID(ctx context.Context, paymentID string) (*paymentdomain.Payment, error) {
	db := persistence.DBFromContext(ctx, r.db)
	var entity PaymentEntity
	err := db.First(&entity, "payment_id = ?", paymentID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, paymentdomain.ErrPaymentNotFound
	}
	if err != nil {
		return nil, err
	}
	return toPaymentDomain(&entity), nil
}

func (r *PaymentRepositoryImpl) FindByOrderID(ctx context.Context, orderID string) (*paymentdomain.Payment, error) {
	db := persistence.DBFromContext(ctx, r.db)
	var entity PaymentEntity
	err := db.First(&entity, "order_id = ?", orderID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, paymentdomain.ErrPaymentNotFound
	}
	if err != nil {
		return nil, err
	}
	return toPaymentDomain(&entity), nil
}

func (r *PaymentRepositoryImpl) FindAll(ctx context.Context) ([]*paymentdomain.Payment, error) {
	db := persistence.DBFromContext(ctx, r.db)
	var entities []PaymentEntity
	err := db.Order("created_at").Find(&entities).Error
	if err != nil {
		return nil, err
	}
	payments := make([]*paymentdomain.Payment, 0, len(entities))
	for i := range entities {
		payments = append(payments, toPaymentDomain(&entities[i]))
	}
	return payments, nil
}

func toPaymentEntity(p *paymentdomain.Payment) *PaymentEntity {
	return &PaymentEntity{
		ID:            p.ID(),
		OrderID:       p.OrderID(),
		Amount:        p.Amount(),
		Status:        string(p.Status()),
		FailureReason: p.FailureReason(),
	}
}

func toPaymentDomain(e *PaymentEntity) *paymentdomain.Payment {
	return paymentdomain.Reconstitute(
		e.ID, e.OrderID, e.Amount,
		paymentdomain.PaymentStatus(e.Status), e.FailureReason,
	)
}
