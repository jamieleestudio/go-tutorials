package application

// GetOrderByIDQuery 按 ID 查询订单（读操作）。
type GetOrderByIDQuery struct {
	OrderID string
}

// GetOrdersByCustomerQuery 按客户查询订单（读操作）。
type GetOrdersByCustomerQuery struct {
	CustomerID string
}
