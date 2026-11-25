// Package domain 订单限界上下文的领域层。
//
// 对应 Java 版 com.example.monolithic.order.domain 包。
// 纯 Go 对象：不依赖 gin / gorm / 平台设施，只依赖 shared 内核
// （由架构守护测试 ArchitectureRulesTest 保证）。
package domain

// OrderStatus 订单状态值对象（枚举），封装订单状态机。
type OrderStatus string

const (
	StatusCreated   OrderStatus = "CREATED"
	StatusPaid      OrderStatus = "PAID"
	StatusConfirmed OrderStatus = "CONFIRMED"
	StatusShipped   OrderStatus = "SHIPPED"
	StatusCancelled OrderStatus = "CANCELLED"
)

// CanTransitionTo 状态机迁移规则（对应 Java 的 switch 表达式）。
func (s OrderStatus) CanTransitionTo(target OrderStatus) bool {
	switch s {
	case StatusCreated:
		return target == StatusPaid || target == StatusCancelled
	case StatusPaid:
		return target == StatusConfirmed || target == StatusCancelled
	case StatusConfirmed:
		return target == StatusShipped
	default: // SHIPPED / CANCELLED：终态
		return false
	}
}
