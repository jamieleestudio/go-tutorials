package rpc_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/jamieleestudio/go-tutorials/architecture/microservices/payment/application/impl"
	"github.com/jamieleestudio/go-tutorials/architecture/microservices/payment/infrastructure"
	paymentrpc "github.com/jamieleestudio/go-tutorials/architecture/microservices/payment/interfaces/rpc"
	"github.com/jamieleestudio/go-tutorials/architecture/microservices/payment/proto/paymentpb"
	"github.com/jamieleestudio/go-tutorials/architecture/microservices/shared/persistence"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// gRPC 契约测试：进程内起真实 gRPC server + 直连 pb client。
// 验证（1）Pay 的分布式幂等；（2）领域错误 → status code 的映射契约 ——
// order 侧客户端按这个契约还原错误。

func startServer(t *testing.T) (string, func()) {
	t.Helper()
	db, err := persistence.Open("sqlite://file:payrpc?mode=memory&cache=shared", "silent")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := persistence.Migrate(db, &infrastructure.PaymentEntity{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	srv := grpc.NewServer()
	appService := impl.NewPaymentServiceImpl(infrastructure.NewPaymentRepository(db))
	paymentpb.RegisterPaymentCommandServiceServer(srv, paymentrpc.NewPaymentRPCServer(appService))

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = srv.Serve(lis) }()
	return lis.Addr().String(), func() {
		srv.Stop()
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
}

func TestPayIsIdempotentOverRPC(t *testing.T) {
	addr, stop := startServer(t)
	defer stop()

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()
	client := paymentpb.NewPaymentCommandServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	first, err := client.Pay(ctx, &paymentpb.PayRequest{OrderId: "o-1", Amount: "99.90"})
	if err != nil {
		t.Fatalf("pay 1: %v", err)
	}
	second, err := client.Pay(ctx, &paymentpb.PayRequest{OrderId: "o-1", Amount: "99.90"})
	if err != nil {
		t.Fatalf("pay 2: %v", err)
	}
	if first.GetPaymentId() != second.GetPaymentId() {
		t.Fatalf("idempotency broken: %s vs %s", first.GetPaymentId(), second.GetPaymentId())
	}
}

func TestRPCErrorContract(t *testing.T) {
	addr, stop := startServer(t)
	defer stop()

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()
	client := paymentpb.NewPaymentCommandServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 404 契约：PAYMENT_NOT_FOUND 前缀 + codes.NotFound（order 侧还原依赖它）
	_, err = client.Refund(ctx, &paymentpb.RefundRequest{PaymentId: "no-such"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("code = %s, want NotFound", status.Code(err))
	}
	if !statusContains(err, "PAYMENT_NOT_FOUND") {
		t.Fatalf("message = %s, want PAYMENT_NOT_FOUND prefix", status.Convert(err).Message())
	}

	// 业务规则契约：PENDING 不能退款 → InvalidArgument INVALID_TRANSITION
	created, err := client.Pay(ctx, &paymentpb.PayRequest{OrderId: "o-2", Amount: "10.00"})
	if err != nil {
		t.Fatalf("pay: %v", err)
	}
	// pay 已直接 SUCCESS，refund 成功一次后再退 → INVALID_TRANSITION
	if _, err := client.Refund(ctx, &paymentpb.RefundRequest{PaymentId: created.GetPaymentId()}); err != nil {
		t.Fatalf("refund: %v", err)
	}
	_, err = client.Refund(ctx, &paymentpb.RefundRequest{PaymentId: created.GetPaymentId()})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %s, want InvalidArgument", status.Code(err))
	}
	if !statusContains(err, "INVALID_TRANSITION") {
		t.Fatalf("message = %s, want INVALID_TRANSITION prefix", status.Convert(err).Message())
	}

	// 金额非法 → InvalidArgument INVALID_PAYMENT
	_, err = client.Pay(ctx, &paymentpb.PayRequest{OrderId: "o-3", Amount: "-5"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %s, want InvalidArgument", status.Code(err))
	}
	if !statusContains(err, "INVALID_PAYMENT") {
		t.Fatalf("message = %s, want INVALID_PAYMENT prefix", status.Convert(err).Message())
	}
}

func statusContains(err error, code string) bool {
	return status.Convert(err).Message()[:len(code)] == code
}
