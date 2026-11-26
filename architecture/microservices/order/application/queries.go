package application

// GetOrderByIDQuery 按订单 ID 查询（读操作）。
type GetOrderByIDQuery struct {
	OrderID string
}

// GetOrdersByCustomerQuery 按客户查询订单（读操作）。
type GetOrdersByCustomerQuery struct {
	CustomerID string
}
