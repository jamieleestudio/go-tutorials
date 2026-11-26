// Package application 支付上下文的应用层：编排用例、管理事务边界。
package application

import (
	"context"

	"github.com/jamieleestudio/go-tutorials/architecture/microservices/payment/api"
	"github.com/shopspring/decimal"
)

// CreatePaymentCommand 创建支付单命令（写操作）。
type CreatePaymentCommand struct {
	OrderID string
	Amount  decimal.Decimal
}

// ProcessPaymentCommand 处理支付命令（写操作，模拟支付网关回调）。
type ProcessPaymentCommand struct {
	PaymentID string
}

// GetPaymentByIDQuery 按支付单 ID 查询（读操作）。
type GetPaymentByIDQuery struct {
	PaymentID string
}

// GetAllPaymentsQuery 查询全部支付单（读操作）。
type GetAllPaymentsQuery struct{}

// PaymentAppService 支付自有用例契约 —— extends 跨服务契约 api.PaymentService。
//
// 对应 Java 的 PaymentAppService extends PaymentService：
//   - 契约方法（Pay/Refund）暴露给 gRPC 接口层，供 order 服务远程调用；
//   - 自有用例只被本服务的 REST 接口层消费。
type PaymentAppService interface {
	api.PaymentService

	CreatePayment(ctx context.Context, cmd CreatePaymentCommand) (*api.PaymentDto, error)
	ProcessPayment(ctx context.Context, cmd ProcessPaymentCommand) (*api.PaymentDto, error)
	GetPaymentByID(ctx context.Context, query GetPaymentByIDQuery) (*api.PaymentDto, error)
	GetAllPayments(ctx context.Context, query GetAllPaymentsQuery) ([]*api.PaymentDto, error)
}
