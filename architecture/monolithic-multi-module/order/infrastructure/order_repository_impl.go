package infrastructure

import (
	"context"
	"errors"

	orderdomain "github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/order/domain"
	"github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/shared/persistence"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 编译期断言：实现领域层定义的仓储端口（依赖倒置的"实现侧"）。
var _ orderdomain.OrderRepository = (*OrderRepositoryImpl)(nil)

// OrderRepositoryImpl 对应 Java 的 OrderRepositoryImpl（JPA → GORM）。
type OrderRepositoryImpl struct{ db *gorm.DB }

func NewOrderRepository(db *gorm.DB) *OrderRepositoryImpl {
	return &OrderRepositoryImpl{db: db}
}

// Save 保存聚合（upsert 语义，对应 Java 版 jpaRepository.save）：
//   已存在 → 按列名显式更新（map 更新会写入零值字段，且不会覆盖 created_at）
//   不存在 → 插入（GORM 自动填充 created_at / updated_at）
//
// 注意：这里故意不用 gorm 的 Save() 做 upsert —— 它对已存在记录走"全字段 UPDATE"，
// 会把未填的 created_at 写成零值 '0000-00-00'，被 MySQL 严格模式拒绝（Error 1292）。
func (r *OrderRepositoryImpl) Save(ctx context.Context, order *orderdomain.Order) error {
	db := persistence.DBFromContext(ctx, r.db)
	entity := toOrderEntity(order)
	return db.Transaction(func(tx *gorm.DB) error {
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
			if err := tx.Omit(clause.Associations).Create(entity).Error; err != nil {
				return err
			}
		}
		// 重建订单行子集合（先删后插）
		if err := tx.Where("order_id = ?", entity.ID).Delete(&OrderItemEntity{}).Error; err != nil {
			return err
		}
		if len(entity.Items) > 0 {
			return tx.Create(&entity.Items).Error
		}
		return nil
	})
}

func (r *OrderRepositoryImpl) FindByID(ctx context.Context, orderID string) (*orderdomain.Order, error) {
	db := persistence.DBFromContext(ctx, r.db)
	var entity OrderEntity
	err := db.Preload("Items").First(&entity, "order_id = ?", orderID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, orderdomain.ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	return toOrderDomain(&entity), nil
}

func (r *OrderRepositoryImpl) FindByCustomerID(ctx context.Context, customerID string) ([]*orderdomain.Order, error) {
	db := persistence.DBFromContext(ctx, r.db)
	var entities []OrderEntity
	err := db.Preload("Items").
		Where("customer_id = ?", customerID).
		Order("created_at").
		Find(&entities).Error
	if err != nil {
		return nil, err
	}
	orders := make([]*orderdomain.Order, 0, len(entities))
	for i := range entities {
		orders = append(orders, toOrderDomain(&entities[i]))
	}
	return orders, nil
}

// --- 领域模型 <-> 持久化实体 转换 ---

func toOrderEntity(order *orderdomain.Order) *OrderEntity {
	items := order.Items()
	entities := make([]OrderItemEntity, 0, len(items))
	for _, it := range items {
		entities = append(entities, OrderItemEntity{
			OrderID:     order.ID(), // 显式设置外键值；绕过关联自动填充（Omit Associations）时 GORM 不会代填
			ProductID:   it.ProductID(),
			ProductName: it.ProductName(),
			Quantity:    it.Quantity(),
			UnitPrice:   it.UnitPrice(),
		})
	}
	return &OrderEntity{
		ID:         order.ID(),
		CustomerID: order.CustomerID(),
		Status:     string(order.Status()),
		PaymentID:  order.PaymentID(),
		Items:      entities,
	}
}

func toOrderDomain(entity *OrderEntity) *orderdomain.Order {
	items := make([]orderdomain.OrderItem, 0, len(entity.Items))
	for _, ie := range entity.Items {
		item, err := orderdomain.NewOrderItem(ie.ProductID, ie.ProductName, ie.Quantity, ie.UnitPrice)
		if err != nil {
			continue // 持久化数据本应始终合法；坏行直接跳过
		}
		items = append(items, item)
	}
	return orderdomain.Reconstitute(
		entity.ID,
		entity.CustomerID,
		items,
		orderdomain.OrderStatus(entity.Status),
		entity.PaymentID,
	)
}
