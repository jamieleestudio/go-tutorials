package application

// GetProductByIDQuery 按产品 ID 查询（读操作）。
type GetProductByIDQuery struct {
	ProductID string
}

// GetAllProductsQuery 查询全部产品（读操作）。
type GetAllProductsQuery struct{}
