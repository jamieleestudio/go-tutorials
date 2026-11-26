package grpc

import (
	"context"

	paymentapi "github.com/jamieleestudio/go-tutorials/architecture/microservices/payment/api"
	"github.com/jamieleestudio/go-tutorials/architecture/microservices/payment/proto/paymentpb"
	productapi "github.com/jamieleestudio/go-tutorials/architecture/microservices/product/api"
	productpb "github.com/jamieleestudio/go-tutorials/architecture/microservices/product/proto/productpb"
	"github.com/shopspring/decimal"
	"github.com/sony/gobreaker"
	"google.golang.org/grpc"
)

// ---- product 契约的 gRPC 实现 ----

var _ productapi.ProductService = (*ProductClient)(nil)

// ProductClient productapi.ProductService 的远程实现。
// application 层像调用本地接口一样调用它 —— RPC 细节被封装在基础设施层。
type ProductClient struct {
	conn    *grpc.ClientConn
	client  productpb.ProductQueryServiceClient
	breaker *gobreaker.CircuitBreaker
}

func NewProductClient(addr string) (*ProductClient, error) {
	conn, err := dial(addr)
	if err != nil {
		return nil, err
	}
	return &ProductClient{
		conn:    conn,
		client:  productpb.NewProductQueryServiceClient(conn),
		breaker: gobreaker.NewCircuitBreaker(BreakerSettings("product")),
	}, nil
}

func (c *ProductClient) Close() error { return c.conn.Close() }

func (c *ProductClient) GetProduct(ctx context.Context, productID string) (*productapi.ProductDto, error) {
	resp, err := remoteCall(ctx, c.breaker,
		func(ctx context.Context) (*productpb.Product, error) {
			return c.client.GetProduct(ctx, &productpb.GetProductRequest{ProductId: productID})
		})
	if err != nil {
		return nil, translateError(err, "Product", "PRODUCT_SERVICE_UNAVAILABLE")
	}
	return productToDto(resp), nil
}

func (c *ProductClient) GetProducts(ctx context.Context, productIDs []string) ([]*productapi.ProductDto, error) {
	resp, err := remoteCall(ctx, c.breaker,
		func(ctx context.Context) (*productpb.ProductList, error) {
			return c.client.GetProducts(ctx, &productpb.GetProductsRequest{ProductIds: productIDs})
		})
	if err != nil {
		return nil, translateError(err, "Product", "PRODUCT_SERVICE_UNAVAILABLE")
	}
	dtos := make([]*productapi.ProductDto, 0, len(resp.GetProducts()))
	for _, p := range resp.GetProducts() {
		dtos = append(dtos, productToDto(p))
	}
	return dtos, nil
}

func productToDto(p *productpb.Product) *productapi.ProductDto {
	price, _ := decimal.NewFromString(p.GetPrice())
	return &productapi.ProductDto{
		ProductID: p.GetProductId(),
		Name:      p.GetName(),
		Price:     price,
		Stock:     int(p.GetStockQuantity()),
	}
}

// ---- payment 契约的 gRPC 实现 ----

var _ paymentapi.PaymentService = (*PaymentClient)(nil)

// PaymentClient paymentapi.PaymentService 的远程实现。
type PaymentClient struct {
	conn    *grpc.ClientConn
	client  paymentpb.PaymentCommandServiceClient
	breaker *gobreaker.CircuitBreaker
}

func NewPaymentClient(addr string) (*PaymentClient, error) {
	conn, err := dial(addr)
	if err != nil {
		return nil, err
	}
	return &PaymentClient{
		conn:    conn,
		client:  paymentpb.NewPaymentCommandServiceClient(conn),
		breaker: gobreaker.NewCircuitBreaker(BreakerSettings("payment")),
	}, nil
}

func (c *PaymentClient) Close() error { return c.conn.Close() }

// Pay 幂等支付：远程端保证同一订单返回同一支付单 ID。
func (c *PaymentClient) Pay(ctx context.Context, orderID string, amount decimal.Decimal) (string, error) {
	resp, err := remoteCall(ctx, c.breaker,
		func(ctx context.Context) (*paymentpb.Payment, error) {
			return c.client.Pay(ctx, &paymentpb.PayRequest{
				OrderId: orderID,
				Amount:  amount.StringFixed(2),
			})
		})
	if err != nil {
		return "", translateError(err, "Payment", "PAYMENT_SERVICE_UNAVAILABLE")
	}
	return resp.GetPaymentId(), nil
}

// Refund 退款。远程 PAYMENT_NOT_FOUND 会还原为 EntityNotFound。
func (c *PaymentClient) Refund(ctx context.Context, paymentID string) error {
	_, err := remoteCall(ctx, c.breaker,
		func(ctx context.Context) (*paymentpb.Payment, error) {
			return c.client.Refund(ctx, &paymentpb.RefundRequest{PaymentId: paymentID})
		})
	if err != nil {
		return translateError(err, "Payment", "PAYMENT_SERVICE_UNAVAILABLE")
	}
	return nil
}
