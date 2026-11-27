// Package application 产品上下文的应用层：编排用例、管理事务边界。
package application

import (
	"context"

	"github.com/jamieleestudio/go-tutorials/architecture/cloud-native/product/api"
	"github.com/shopspring/decimal"
)

// CreateProductCommand 创建产品命令（写操作）。
type CreateProductCommand struct {
	Name  string
	Price decimal.Decimal
	Stock int
}

// DeductStockCommand 扣减库存命令（写操作）。
type DeductStockCommand struct {
	ProductID string
	Quantity  int
}

// GetProductByIDQuery 按产品 ID 查询（读操作）。
type GetProductByIDQuery struct {
	ProductID string
}

// GetAllProductsQuery 查询全部产品（读操作）。
type GetAllProductsQuery struct{}

// ProductAppService 产品自有用例契约 —— extends 跨服务契约 api.ProductService。
//
// 对应 Java 的 ProductAppService extends ProductService：
//   - 契约方法（GetProduct/GetProducts）同时暴露给 gRPC 接口层，供 order 服务远程调用；
//   - 自有用例只被本服务的 REST 接口层消费。
//
// 接口层依赖本接口（而非 impl）—— provider-defined interface 是熔断降级
// 与测试替身（fake 注入）能成立的关键。
type ProductAppService interface {
	api.ProductService

	CreateProduct(ctx context.Context, cmd CreateProductCommand) (*api.ProductDto, error)
	DeductStock(ctx context.Context, cmd DeductStockCommand) error
	GetProductByID(ctx context.Context, query GetProductByIDQuery) (*api.ProductDto, error)
	GetAllProducts(ctx context.Context, query GetAllProductsQuery) ([]*api.ProductDto, error)
}
