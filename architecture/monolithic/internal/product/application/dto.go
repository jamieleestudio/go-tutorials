package application

import "github.com/shopspring/decimal"

// ProductDto 产品出参（应用层输出契约）。
// 也是跨上下文的对外契约：Order 上下文只允许依赖本包（读 DTO，不读领域模型）。
type ProductDto struct {
	ProductID string
	Name      string
	Price     decimal.Decimal
	Stock     int
}
