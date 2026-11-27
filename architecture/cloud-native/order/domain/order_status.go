package domain

// OrderStatus 订单状态值对象 —— 封装订单状态机（对应 Java 的 OrderStatus 枚举）。
type OrderStatus string

const (
	StatusCreated   OrderStatus = "CREATED"
	StatusPaid      OrderStatus = "PAID"
	StatusConfirmed OrderStatus = "CONFIRMED"
	StatusShipped   OrderStatus = "SHIPPED"
	StatusCancelled OrderStatus = "CANCELLED"
)

// CanTransitionTo 状态机规则：
//
//	CREATED → PAID | CANCELLED
//	PAID    → CONFIRMED | CANCELLED
//	CONFIRMED → SHIPPED
//	SHIPPED / CANCELLED 为终态
func (s OrderStatus) CanTransitionTo(target OrderStatus) bool {
	switch s {
	case StatusCreated:
		return target == StatusPaid || target == StatusCancelled
	case StatusPaid:
		return target == StatusConfirmed || target == StatusCancelled
	case StatusConfirmed:
		return target == StatusShipped
	default:
		return false
	}
}
