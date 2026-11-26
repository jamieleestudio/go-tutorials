package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/product/domain"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared/persistence"
	"gorm.io/gorm"
)

var _ domain.DeductedOrderRepository = (*DeductedOrderRepositoryImpl)(nil)

// DeductedOrderEntity Saga 补偿记账实体（一行 = 一条扣减记录）。
//
// Java 版是 (order_id, product_id, quantity) 明细行；Go 版用 JSON 列存放
// 订单的全部行 —— 读写都以订单为单位，没有跨行查询需求。
type DeductedOrderEntity struct {
	OrderID   string `gorm:"column:order_id;size:36;primaryKey"`
	Lines     string `gorm:"column:lines;type:text;not null"` // JSON: [{productId,quantity}]
	CreatedAt time.Time
}

func (DeductedOrderEntity) TableName() string { return "deducted_orders" }

// DeductedOrderRepositoryImpl 补偿记账仓储实现（GORM）。
type DeductedOrderRepositoryImpl struct{ db *gorm.DB }

func NewDeductedOrderRepository(db *gorm.DB) *DeductedOrderRepositoryImpl {
	return &DeductedOrderRepositoryImpl{db: db}
}

func (r *DeductedOrderRepositoryImpl) Save(ctx context.Context, record *domain.DeductedOrderRecord) error {
	db := persistence.DBFromContext(ctx, r.db)
	entity, err := toDeductedEntity(record)
	if err != nil {
		return err
	}
	return db.Save(entity).Error
}

func (r *DeductedOrderRepositoryImpl) FindByID(ctx context.Context, orderID string) (*domain.DeductedOrderRecord, error) {
	db := persistence.DBFromContext(ctx, r.db)
	var entity DeductedOrderEntity
	err := db.First(&entity, "order_id = ?", orderID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil // 应用层决定"没记账"怎么处理（回补时跳过）
	}
	if err != nil {
		return nil, err
	}
	return toDeductedRecord(&entity)
}

func (r *DeductedOrderRepositoryImpl) Delete(ctx context.Context, orderID string) error {
	db := persistence.DBFromContext(ctx, r.db)
	return db.Delete(&DeductedOrderEntity{}, "order_id = ?", orderID).Error
}

func toDeductedEntity(record *domain.DeductedOrderRecord) (*DeductedOrderEntity, error) {
	lines, err := json.Marshal(record.Lines)
	if err != nil {
		return nil, err
	}
	return &DeductedOrderEntity{
		OrderID:   record.OrderID,
		Lines:     string(lines),
		CreatedAt: time.Now(),
	}, nil
}

func toDeductedRecord(entity *DeductedOrderEntity) (*domain.DeductedOrderRecord, error) {
	var lines []domain.DeductedLine
	if err := json.Unmarshal([]byte(entity.Lines), &lines); err != nil {
		return nil, err
	}
	return &domain.DeductedOrderRecord{OrderID: entity.OrderID, Lines: lines}, nil
}
