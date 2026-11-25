package application

// GetPaymentByIDQuery 按支付单 ID 查询（读操作）。
type GetPaymentByIDQuery struct {
	PaymentID string
}

// GetAllPaymentsQuery 查询全部支付单（读操作）。
type GetAllPaymentsQuery struct{}
