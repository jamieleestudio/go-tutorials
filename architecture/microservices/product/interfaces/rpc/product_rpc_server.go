// Package rpc product 服务的 gRPC 接口层（对应 Java 的 ProductRpcController）。
//
// 职责：把 pb.ProductQueryServiceServer（protobuf 契约）适配到本服务的
// api.ProductService（Go 契约）—— 接口层调用应用层接口，不感知领域内部。
//
// 同时负责错误翻译：shared.DomainError → gRPC status code。
package rpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/jamieleestudio/go-tutorials/architecture/microservices/product/api"
	"github.com/jamieleestudio/go-tutorials/architecture/microservices/product/proto/productpb"
	"github.com/jamieleestudio/go-tutorials/architecture/microservices/shared"
)

// ProductRPCServer gRPC server 适配器（实现 pb 生成的 server 接口）。
type ProductRPCServer struct {
	productpb.UnimplementedProductQueryServiceServer

	// 依赖最小契约 api.ProductService（对应 Java RpcController 依赖 ProductService）。
	productService api.ProductService
}

func NewProductRPCServer(productService api.ProductService) *ProductRPCServer {
	return &ProductRPCServer{productService: productService}
}

func (s *ProductRPCServer) GetProduct(ctx context.Context, req *productpb.GetProductRequest) (*productpb.Product, error) {
	dto, err := s.productService.GetProduct(ctx, req.GetProductId())
	if err != nil {
		return nil, toRPCError(err)
	}
	return productToProto(dto), nil
}

func (s *ProductRPCServer) GetProducts(ctx context.Context, req *productpb.GetProductsRequest) (*productpb.ProductList, error) {
	dtos, err := s.productService.GetProducts(ctx, req.GetProductIds())
	if err != nil {
		return nil, toRPCError(err)
	}
	products := make([]*productpb.Product, 0, len(dtos))
	for _, dto := range dtos {
		products = append(products, productToProto(dto))
	}
	return &productpb.ProductList{Products: products}, nil
}

func productToProto(dto *api.ProductDto) *productpb.Product {
	return &productpb.Product{
		ProductId:     dto.ProductID,
		Name:          dto.Name,
		Price:         dto.Price.StringFixed(2),
		StockQuantity: int32(dto.Stock),
	}
}

// toRPCError 领域错误 → gRPC status：
//   - ENTITY_NOT_FOUND        → codes.NotFound（客户端可还原业务语义）
//   - BUSINESS_RULE_VIOLATION → codes.InvalidArgument
//
// code 放在 status message 前缀里，客户端按 ":" 切分还原错误码。
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
