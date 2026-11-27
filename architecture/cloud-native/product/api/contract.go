// Package api 是 product 上下文的跨服务契约包（对应 Java 的 ms-product-api 模块）。
//
// 这里是 order 服务"被允许看到"的全部内容：ProductService 接口 + ProductDto。
// 两个服务各自独立部署，所以这个契约通过 HTTP + JSON 落地：
//   - product 服务端：interfaces/web 的 RPC 端点（/rpc/products）把本接口暴露为 HTTP；
//   - order 消费端：infrastructure/rpc 的 HTTP client 适配为本接口。
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
	ProductID string          `json:"productId"`
	Name      string          `json:"name"`
	Price     decimal.Decimal `json:"price"`
	Stock     int             `json:"stock"`
}

// ProductService 商品提供者契约 —— 只含跨上下文方法（CROSS-CONTEXT methods only）。
//
// 被远程的 order 服务消费：Java 版经 OpenFeign 走 HTTP，Go 版经
// infrastructure/rpc 的 HTTP 客户端走同样的端点（/rpc/products）。
// P7 的服务发现是 K8s Service DNS，不引入 gRPC/注册中心，照旧展示
// "朴素 HTTP RPC"下的幂等/超时/重试。
type ProductService interface {
	// GetProduct 按 ID 查询单个商品；不存在时返回 PRODUCT_NOT_FOUND。
	GetProduct(ctx context.Context, productID string) (*ProductDto, error)

	// GetProducts 按 ID 批量查询。与 Java 一致：不存在的 ID 不报错，
	// 只返回能找到的部分 —— 由 order 侧自行判断缺货缺品。
	GetProducts(ctx context.Context, productIDs []string) ([]*ProductDto, error)
}
