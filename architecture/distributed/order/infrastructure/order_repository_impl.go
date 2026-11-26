package infrastructure

import (
	"context"
	"errors"

	"github.com/jamieleestudio/go-tutorials/architecture/distributed/order/domain"
	"github.com/jamieleestudio/go-tutorials/architecture/distributed/shared/persistence"
	"gorm.io/gorm"
)

var _ domain.OrderRepository = (*OrderRepositoryImpl)(nil)

// OrderRepositoryImpl 订单仓储实现（GORM）。
type OrderRepositoryImpl struct{ db *gorm.DB }

func NewOrderRepository(db *gorm.DB) *OrderRepositoryImpl {
	return &OrderRepositoryImpl{db: db}
}

// Save 保存订单（聚合整体落库）。
//
// 为什么不用 gorm 的 Save() 做 upsert：Save() 按主键存在与否更新/插入，
// 但它会把【所有字段】包括零值一起写入，并且对 created_at 这类由 GORM
// 管理的列处理不直观；这里改为显式 UPDATE 指定列 + RowsAffected==0 时 INSERT，
// 语义与 Java 版 JPA save（合并语义）对齐。订单行则整体删除重插（订单行不可变，
// 重插保证与聚合状态严格一致）。
func (r *OrderRepositoryImpl) Save(ctx context.Context, order *domain.Order) error {
	db := persistence.DBFromContext(ctx, r.db)
	return db.Transaction(func(tx *gorm.DB) error {
		entity := toOrderEntity(order)
		res := tx.Model(&OrderEntity{}).
			Where("order_id = ?", entity.ID).
			Updates(map[string]any{
				"customer_id": entity.CustomerID,
				"status":      entity.Status,
				"payment_id":  entity.PaymentID,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			if err := tx.Create(entity).Error; err != nil {
				return err
			}
		}

		if err := tx.Where("order_id = ?", entity.ID).Delete(&OrderItemEntity{}).Error; err != nil {
			return err
		}
		items := toOrderItemEntities(order)
		if len(items) > 0 {
			return tx.Create(&items).Error
		}
		return nil
	})
}

func (r *OrderRepositoryImpl) FindByID(ctx context.Context, orderID string) (*domain.Order, error) {
	db := persistence.DBFromContext(ctx, r.db)
	var entity OrderEntity
	err := db.First(&entity, "order_id = ?", orderID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	var itemEntities []OrderItemEntity
	if err := db.Where("order_id = ?", orderID).Order("id").Find(&itemEntities).Error; err != nil {
		return nil, err
	}
	return toOrderDomain(&entity, itemEntities), nil
}

func (r *OrderRepositoryImpl) FindByCustomerID(ctx context.Context, customerID string) ([]*domain.Order, error) {
	db := persistence.DBFromContext(ctx, r.db)
	var entities []OrderEntity
	err := db.Where("customer_id = ?", customerID).Order("created_at").Find(&entities).Error
	if err != nil {
		return nil, err
	}
	orders := make([]*domain.Order, 0, len(entities))
	for i := range entities {
		var itemEntities []OrderItemEntity
		if err := db.Where("order_id = ?", entities[i].ID).Order("id").Find(&itemEntities).Error; err != nil {
			return nil, err
		}
		orders = append(orders, toOrderDomain(&entities[i], itemEntities))
	}
	return orders, nil
}

func toOrderEntity(o *domain.Order) *OrderEntity {
	return &OrderEntity{
		ID:         o.ID(),
		CustomerID: o.CustomerID(),
		Status:     string(o.Status()),
		PaymentID:  o.PaymentID(),
	}
}

func toOrderItemEntities(o *domain.Order) []OrderItemEntity {
	items := o.Items()
	entities := make([]OrderItemEntity, 0, len(items))
	for _, item := range items {
		entities = append(entities, OrderItemEntity{
			OrderID:     o.ID(),
			ProductID:   item.ProductID(),
			ProductName: item.ProductName(),
			Quantity:    item.Quantity(),
			UnitPrice:   item.UnitPrice(),
		})
	}
	return entities
}

func toOrderDomain(e *OrderEntity, itemEntities []OrderItemEntity) *domain.Order {
	items := make([]*domain.OrderItem, 0, len(itemEntities))
	for i := range itemEntities {
		ie := &itemEntities[i]
		item, err := domain.NewOrderItem(ie.ProductID, ie.ProductName, ie.Quantity, ie.UnitPrice)
		if err != nil {
			// 持久化数据违反领域不变量：这是 bug 而非业务错误，直接 panic 升级
			panic(err)
		}
		items = append(items, item)
	}
	return domain.Reconstitute(e.ID, e.CustomerID, items, domain.OrderStatus(e.Status), e.PaymentID)
}
