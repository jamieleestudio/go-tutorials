package application

import "context"

// ProductService 产品应用服务契约。
// 被【本上下文的接口层】和【Order 上下文的跨上下文调用】消费。
type ProductService interface {
	CreateProduct(ctx context.Context, cmd CreateProductCommand) (*ProductDto, error)
	DeductStock(ctx context.Context, cmd DeductStockCommand) error
	GetProductByID(ctx context.Context, query GetProductByIDQuery) (*ProductDto, error)
	GetAllProducts(ctx context.Context, query GetAllProductsQuery) ([]ProductDto, error)

	// --- 跨上下文方法（供 Order 上下文调用） ---

	GetProduct(ctx context.Context, productID string) (*ProductDto, error)
	GetProducts(ctx context.Context, productIDs []string) ([]ProductDto, error)
}
