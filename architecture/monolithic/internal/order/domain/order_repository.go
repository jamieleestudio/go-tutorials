package domain

import (
	"context"
	"errors"
)

// ErrOrderNotFound 是领域层的哨兵错误（sentinel error）：
// 由仓储实现返回，应用层再转换为 shared.EntityNotFound。
// 这对应 Java 版 Optional<Order> + orElseThrow 的组合。
var ErrOrderNotFound = errors.New("order not found")

// OrderRepository 订单聚合的仓储端口（Repository Port）。
//
// 领域层定义接口，基础设施层提供实现 —— 依赖倒置（依赖倒置原则 DIP）。
// Java 版返回 Optional<T>；Go 惯例是 (T, error)，未找到时返回 ErrOrderNotFound。
type OrderRepository interface {
	Save(ctx context.Context, order *Order) error
	FindByID(ctx context.Context, orderID string) (*Order, error)
	FindByCustomerID(ctx context.Context, customerID string) ([]*Order, error)
}
