package domain

import "context"

// DeductedOrderRecord Saga 补偿记账：某订单被扣了什么。
// 订单取消时凭它精确回补 —— 这是"记账 → 凭据补偿"模式。
type DeductedOrderRecord struct {
	OrderID string
	Lines   []DeductedLine
}

type DeductedLine struct {
	ProductID string
	Quantity  int
}

// DeductedOrderRepository 补偿记账端口（对应 Java 的 DeductedOrderRepository，
// record 嵌套在端口里，Go 版拆成两个类型）。
type DeductedOrderRepository interface {
	Save(ctx context.Context, record *DeductedOrderRecord) error
	// FindByID 找不到时返回 (nil, nil)，由应用层决定如何处理。
	FindByID(ctx context.Context, orderID string) (*DeductedOrderRecord, error)
	Delete(ctx context.Context, orderID string) error
}
