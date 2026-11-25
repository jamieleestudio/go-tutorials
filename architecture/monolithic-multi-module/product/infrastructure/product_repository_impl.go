package infrastructure

import (
	"context"
	"errors"

	"github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/shared/persistence"
	productdomain "github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/product/domain"
	"gorm.io/gorm"
)

var _ productdomain.ProductRepository = (*ProductRepositoryImpl)(nil)

// ProductRepositoryImpl 产品仓储实现（GORM）。
type ProductRepositoryImpl struct{ db *gorm.DB }

func NewProductRepository(db *gorm.DB) *ProductRepositoryImpl {
	return &ProductRepositoryImpl{db: db}
}

// Save 保存产品（upsert 语义）：
//   已存在 → 按列名显式更新（不碰 created_at，零值字段也会写入）
//   不存在 → 插入（GORM 自动填充 created_at / updated_at）
// 不用 gorm 的 Save() 做 upsert 的原因见 order_repository_impl.go。
func (r *ProductRepositoryImpl) Save(ctx context.Context, product *productdomain.Product) error {
	db := persistence.DBFromContext(ctx, r.db)
	entity := toProductEntity(product)
	res := db.Model(&ProductEntity{}).
		Where("product_id = ?", entity.ID).
		Updates(map[string]any{
			"name":  entity.Name,
			"price": entity.Price,
			"stock": entity.Stock,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return db.Create(entity).Error
	}
	return nil
}

func (r *ProductRepositoryImpl) FindByID(ctx context.Context, productID string) (*productdomain.Product, error) {
	db := persistence.DBFromContext(ctx, r.db)
	var entity ProductEntity
	err := db.First(&entity, "product_id = ?", productID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, productdomain.ErrProductNotFound
	}
	if err != nil {
		return nil, err
	}
	return toProductDomain(&entity), nil
}

func (r *ProductRepositoryImpl) FindAllByIDs(ctx context.Context, productIDs []string) ([]*productdomain.Product, error) {
	if len(productIDs) == 0 {
		return nil, nil
	}
	db := persistence.DBFromContext(ctx, r.db)
	var entities []ProductEntity
	err := db.Where("product_id IN ?", productIDs).Find(&entities).Error
	if err != nil {
		return nil, err
	}
	return toProductDomains(entities), nil
}

func (r *ProductRepositoryImpl) FindAll(ctx context.Context) ([]*productdomain.Product, error) {
	db := persistence.DBFromContext(ctx, r.db)
	var entities []ProductEntity
	err := db.Order("created_at").Find(&entities).Error
	if err != nil {
		return nil, err
	}
	return toProductDomains(entities), nil
}

func toProductEntity(p *productdomain.Product) *ProductEntity {
	return &ProductEntity{
		ID:    p.ID(),
		Name:  p.Name(),
		Price: p.Price(),
		Stock: p.Stock(),
	}
}

func toProductDomain(e *ProductEntity) *productdomain.Product {
	return productdomain.Reconstitute(e.ID, e.Name, e.Price, e.Stock)
}

func toProductDomains(entities []ProductEntity) []*productdomain.Product {
	products := make([]*productdomain.Product, 0, len(entities))
	for i := range entities {
		products = append(products, toProductDomain(&entities[i]))
	}
	return products
}
