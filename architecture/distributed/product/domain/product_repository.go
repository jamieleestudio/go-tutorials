package domain

import (
	"context"
	"errors"
)

// ErrProductNotFound 产品不存在的哨兵错误（由仓储实现返回，应用层转换）。
var ErrProductNotFound = errors.New("product not found")

// ProductRepository 产品聚合的仓储端口。
type ProductRepository interface {
	Save(ctx context.Context, product *Product) error
	FindByID(ctx context.Context, productID string) (*Product, error)
	FindAllByIDs(ctx context.Context, productIDs []string) ([]*Product, error)
	FindAll(ctx context.Context) ([]*Product, error)
}
