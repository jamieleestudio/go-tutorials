// Package api 是 product 上下文的跨服务契约包（对应 Java 的 ms-product-api 模块）。
//
// 这里是 order 服务"被允许看到"的全部内容：ProductService 接口 + ProductDto。
// 两个服务各自独立部署，所以这个契约通过 gRPC + protobuf 落地：
//   - product 服务端：interfaces/rpc 把本接口适配为 gRPC server；
//   - order 消费端：infrastructure/grpc 把 gRPC client 适配为本接口。
//
// 铁律（架构守护测试保证）：契约包只依赖 shared，不依赖本服务的
// domain / application / infrastructure / interfaces 任何内部包。
package api

import (
	"context"

	"github.com/shopspring/decimal"
)

// ProductDto 商品出参契约（对应 Java product-api 的 ProductDto record）。
//
// 与 Java 一致：跨服务 DTO 就是应用层的输出契约，本服务的应用层直接返回它
// （没有第二套"自有 DTO"—— 提供者即契约的实现者）。
type ProductDto struct {
	ProductID string
	Name      string
	Price     decimal.Decimal
	Stock     int
}

// ProductService 商品提供者契约 —— 只含跨上下文方法（CROSS-CONTEXT methods only）。
//
// 被远程的 order 服务消费：
//   - Java 版经 OpenFeign 走 HTTP；
//   - Go 版经 gRPC（契约的 protobuf 形态在 product/proto/productpb）。
type ProductService interface {
	// GetProduct 按 ID 查询单个商品；不存在时返回 PRODUCT_NOT_FOUND。
	GetProduct(ctx context.Context, productID string) (*ProductDto, error)

	// GetProducts 按 ID 批量查询。与 Java 一致：不存在的 ID 不报错，
	// 只返回能找到的部分 —— 由 order 侧自行判断缺货缺品。
	GetProducts(ctx context.Context, productIDs []string) ([]*ProductDto, error)
}
