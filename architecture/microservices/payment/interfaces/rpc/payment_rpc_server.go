// Package rpc payment 服务的 gRPC 接口层（对应 Java 的 PaymentRpcController）。
//
// 职责：把 pb.PaymentCommandServiceServer（protobuf 契约）适配到本服务的
// api.PaymentService —— 接口层调用应用层契约接口，不感知领域内部。
package rpc

import (
	"context"
	"errors"

	"github.com/shopspring/decimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/jamieleestudio/go-tutorials/architecture/microservices/payment/api"
	"github.com/jamieleestudio/go-tutorials/architecture/microservices/payment/proto/paymentpb"
	"github.com/jamieleestudio/go-tutorials/architecture/microservices/shared"
)

// PaymentRPCServer gRPC server 适配器（实现 pb 生成的 server 接口）。
type PaymentRPCServer struct {
	paymentpb.UnimplementedPaymentCommandServiceServer

	paymentService api.PaymentService
}

func NewPaymentRPCServer(paymentService api.PaymentService) *PaymentRPCServer {
	return &PaymentRPCServer{paymentService: paymentService}
}

// Pay 幂等支付：同一订单重复 Pay 返回同一支付单 ID。
func (s *PaymentRPCServer) Pay(ctx context.Context, req *paymentpb.PayRequest) (*paymentpb.Payment, error) {
	amount, err := decimal.NewFromString(req.GetAmount())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "INVALID_AMOUNT: %v", err)
	}
	paymentID, err := s.paymentService.Pay(ctx, req.GetOrderId(), amount)
	if err != nil {
		return nil, toRPCError(err)
	}
	// Pay 契约只返回 paymentID（与 Java 一致）；DTO 侧回查一次订单的支付单
	// 用于拼 protobuf 响应。这里直接用 ID 构造最小响应（避免二次回查）。
	return &paymentpb.Payment{PaymentId: paymentID}, nil
}

func (s *PaymentRPCServer) Refund(ctx context.Context, req *paymentpb.RefundRequest) (*paymentpb.Payment, error) {
	if err := s.paymentService.Refund(ctx, req.GetPaymentId()); err != nil {
		return nil, toRPCError(err)
	}
	return &paymentpb.Payment{PaymentId: req.GetPaymentId()}, nil
}

// toRPCError 领域错误 → gRPC status（与 product 侧一致）。
func toRPCError(err error) error {
	var domErr *shared.DomainError
	if errors.As(err, &domErr) {
		code := codes.InvalidArgument
		if domErr.Kind == shared.KindEntityNotFound {
			code = codes.NotFound
		}
		return status.Errorf(code, "%s: %s", domErr.Code, domErr.Message)
	}
	return status.Errorf(codes.Internal, "INTERNAL_ERROR: %v", err)
}
