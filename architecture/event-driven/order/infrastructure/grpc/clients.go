// Package grpc 跨服务契约的 gRPC 适配器（对应 Java 的 RpcClient / Feign）。
//
// P5 的 Java 版 RpcClient 没有熔断（P4 才引入 resilience4j），Go 版同样
// 保持轻量：直连 + 超时。熔断是演进点（见 P4 microservices 的 gobreaker
// 实现与 README）。
package grpc

import (
	"context"
	"time"

	productapi "github.com/jamieleestudio/go-tutorials/architecture/event-driven/product/api"
	productpb "github.com/jamieleestudio/go-tutorials/architecture/event-driven/product/proto/productpb"
	"github.com/shopspring/decimal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// 跨服务同步调用的超时预算（对应 Java RpcClient 的超时配置）。
const rpcTimeout = 3 * time.Second

var _ productapi.ProductService = (*ProductClient)(nil)

// ProductClient productapi.ProductService 的远程实现。
// application 层像调用本地接口一样调用它 —— RPC 细节被封装在基础设施层。
type ProductClient struct {
	conn   *grpc.ClientConn
	client productpb.ProductQueryServiceClient
}

func NewProductClient(addr string) (*ProductClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &ProductClient{
		conn:   conn,
		client: productpb.NewProductQueryServiceClient(conn),
	}, nil
}

func (c *ProductClient) Close() error { return c.conn.Close() }

func (c *ProductClient) GetProduct(ctx context.Context, productID string) (*productapi.ProductDto, error) {
	callCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()
	resp, err := c.client.GetProduct(callCtx, &productpb.GetProductRequest{ProductId: productID})
	if err != nil {
		return nil, err
	}
	return productToDto(resp), nil
}

func (c *ProductClient) GetProducts(ctx context.Context, productIDs []string) ([]*productapi.ProductDto, error) {
	callCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()
	resp, err := c.client.GetProducts(callCtx, &productpb.GetProductsRequest{ProductIds: productIDs})
	if err != nil {
		return nil, err
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
