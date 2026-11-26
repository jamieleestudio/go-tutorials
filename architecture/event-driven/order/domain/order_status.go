package domain

// OrderStatus Saga 世界的订单状态：支付与库存【异步】进行，
// 没有 PAID 中间态 —— 双条件到齐直接 CONFIRMED。
type OrderStatus string

const (
	StatusCreated   OrderStatus = "CREATED"
	StatusConfirmed OrderStatus = "CONFIRMED"
	StatusCancelled OrderStatus = "CANCELLED"
)

// CanTransitionTo 状态机守卫：
// CREATED → CANCELLED（补偿）；CONFIRMED / CANCELLED 是终态。
func (s OrderStatus) CanTransitionTo(target OrderStatus) bool {
	switch s {
	case StatusCreated:
		return target == StatusCancelled
	default:
		return false
	}
}
