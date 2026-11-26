// Package application 产品上下文的应用层：编排用例、管理事务边界。
package application

import (
	"context"

	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/product/api"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared/event"
	"github.com/shopspring/decimal"
)

// CreateProductCommand 创建产品命令（写操作）。
type CreateProductCommand struct {
	Name  string
	Price decimal.Decimal
	Stock int
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
//   - 契约方法（GetProduct/GetProducts）暴露给 gRPC 接口层，供 order 远程调用；
//   - Saga 反应方法（DeductForOrder/RestockForOrder）暴露给 messaging 接口层；
//   - 自有用例只被本服务的 REST 接口层消费。
//
// 接口层依赖本接口（而非 impl）—— 测试替身（fake 注入）能成立的关键。
type ProductAppService interface {
	api.ProductService

	CreateProduct(ctx context.Context, cmd CreateProductCommand) (*api.ProductDto, error)
	GetProductByID(ctx context.Context, query GetProductByIDQuery) (*api.ProductDto, error)
	GetAllProducts(ctx context.Context, query GetAllProductsQuery) ([]*api.ProductDto, error)

	// DeductForOrder Saga 反应：OrderCreatedEvent → 预校验全部行后扣减库存
	// → 补偿记账 → 发布 InventoryDeductedEvent（缺货/缺品则发布 InventoryDeductFailedEvent）。
	//
	// eventID 用于消费端幂等（at-least-once 投递下重复到达必须跳过）。
	DeductForOrder(ctx context.Context, eventID string, evt *event.OrderCreatedEvent) error

	// RestockForOrder Saga 补偿反应：OrderCancelledEvent → 凭记账回补已扣库存。
	RestockForOrder(ctx context.Context, eventID string, orderID string) error
}
