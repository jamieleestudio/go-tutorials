package domain

import (
	"context"
	"errors"
)

// ErrOrderNotFound 订单不存在的哨兵错误（由仓储实现返回，应用层转换）。
var ErrOrderNotFound = errors.New("order not found")

// OrderRepository 订单聚合的仓储端口。
// 领域定义接口，基础设施层提供实现（依赖方向：infrastructure → domain）。
type OrderRepository interface {
	Save(ctx context.Context, order *Order) error
	FindByID(ctx context.Context, orderID string) (*Order, error)
	FindByCustomerID(ctx context.Context, customerID string) ([]*Order, error)
}
