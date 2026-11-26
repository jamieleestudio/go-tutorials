# 微服务架构 (Microservices Architecture)

## 概述

裸分布式（P2 三上下文同进程）的**进程拆分 + 治理化**：每个上下文独立进程、独立数据库，gin 网关统一入口（8080），跨服务调用走 **gRPC + protobuf**（对应 Java 版 OpenFeign），order 侧叠加 **gobreaker 熔断 + 3s deadline**（对应 Resilience4j + Feign 超时）。

## 与 Java 版的核心差异（治理生态的 Go 对应物）

| 能力 | ☕ Java 微服务 | 🐹 Go 微服务 |
|---|---|---|
| RPC | OpenFeign 声明式（HTTP + JSON） | **gRPC + protobuf**（强契约、HTTP/2 二进制） |
| 跨服务契约 | provider 定义的 api 模块（`PaymentService`） | 同样是 provider 定义的 `api` 包 + **proto 契约**（pb 代码提交入库） |
| 网关 | Spring Cloud Gateway（8080，`lb://` 路由） | gin + `httputil.ReverseProxy`（8080，前缀路由） |
| 服务发现 | **Nacos** 注册中心 | **env 固定地址**（`PRODUCT_GRPC_ADDR` 等）—— 演进点见下 |
| 负载均衡 | Spring Cloud LoadBalancer（客户端） | 无（单实例部署；多实例随服务发现一并演进） |
| 熔断 | Resilience4j（滑动窗口/半开/超时） | **gobreaker**（失败率≥50% 且样本≥10 打开，open 10s 半开 3 探测） |
| 超时 | Resilience4j TimeLimiter 3s | gRPC deadline 3s（`remoteTimeout`） |
| 降级 | Feign fallback 抛 `*_SERVICE_UNAVAILABLE` | 同语义：`PAYMENT_SERVICE_UNAVAILABLE` / `PRODUCT_SERVICE_UNAVAILABLE` |
| 配置中心 | Nacos optional 导入 | env 变量（每服务 `app.DefaultConfig()`） |
| 链路追踪 | Micrometer Tracing + Zipkin | 无 —— 演进点（接缝：错误/日志已结构化） |

### 与 Java 版的一处有意改进

Java 的 Feign fallback 会吞掉**所有**远程错误（包括 404）。Go 版在进入熔断/fallback 之前先识别 `status.NotFound`，还原为 `ENTITY_NOT_FOUND`（如 `PRODUCT_NOT_FOUND`）—— 保留"资源不存在"的业务语义；只有基础设施性失败（网络/超时/熔断打开）才降级为 `*_SERVICE_UNAVAILABLE`。

## 模块结构（5 个 Go module）

```
microservices/
├── shared/                  共享内核：DomainError + AggregateRoot + EventBus + ID + TxRunner
│   └── persistence/         （子包例外）GORM 事务传播 + DB 打开/协议路由
├── product/                 商品服务（提供方）
│   ├── api/                 跨服务契约：ProductService + ProductDto（order 唯一可见面）
│   ├── proto/               product.proto —— GetProduct/GetProducts
│   │   └── productpb/       pb 生成代码（提交入库）
│   ├── domain/ + application/ (+impl) + infrastructure/ + interfaces/(web|rpc|httpmw)
│   └── app/                 组合根：REST :8083 + gRPC :9183
├── payment/                 支付服务（提供方，同构）
│   ├── api/                 PaymentService{Pay 幂等, Refund} + PaymentDto
│   ├── proto/               payment.proto —— Pay/Refund
│   ├── ... 4 层
│   └── app/                 REST :8082 + gRPC :9182
├── order/                   订单服务（消费方，无自己的 gRPC server）
│   ├── domain/              Order 聚合 + 状态机（CREATED→PAID→CONFIRMED→SHIPPED / CANCELLED）+ 3 领域事件
│   ├── application/         编排 + EventPublisherService 端口
│   ├── infrastructure/
│   │   ├── persistence/     GORM（orders + order_items）
│   │   ├── grpc/            Feign 对应物：契约客户端 + gobreaker + deadline + fallback
│   │   └── event/           EventBus 发布者 + 监听器
│   ├── interfaces/          web(/api/orders) + event(监听 OrderCreatedEvent 打日志) + httpmw
│   └── app/                 REST :8081，连 product/payment 的 gRPC
└── gateway/                 API 网关：8080，/api/{orders,payments,products} → 上游
```

### 端口与环境变量（对应 Java application.yml）

| 服务 | HTTP | gRPC | 关键 env |
|---|---|---|---|
| gateway | 8080 | — | `ORDER_SERVICE_URL` / `PAYMENT_SERVICE_URL` / `PRODUCT_SERVICE_URL` |
| order | 8081 | —（纯消费方） | `PRODUCT_GRPC_ADDR`(:9183) / `PAYMENT_GRPC_ADDR`(:9182) / `DB_DSN` |
| payment | 8082 | 9182 | `DB_DSN` |
| product | 8083 | 9183 | `DB_DSN` |

默认全部 SQLite 内存库（对应 Java 版 H2 mem），`DB_DSN` 可切 MySQL：
`DB_DSN=monolith:monolith@tcp(localhost:13306)/ms_product`（每服务独立库）。

## RPC 契约演进（与 Java 版逐行对照）

```
☕: PaymentServiceRpcClient（RestClient + 手写 URL + 固定地址）
    ↓ 微服务化
☕: PaymentClient(@FeignClient name="payment-service") → Nacos + LB + Resilience4j + fallback
    ↓ Go
🐹: order/infrastructure/grpc.PaymentClient implements paymentapi.PaymentService
      → proto/paymentpb.PaymentCommandServiceClient（gRPC）+ gobreaker + 3s deadline + fallback
```

`OrderServiceImpl` 依然**零修改**地编排 —— 依赖倒置贯穿「裸→微」两阶段。这就是 provider 定义契约（`api` 包）的价值：应用层眼里只有本地接口，RPC 是基础设施适配细节。

### protobuf 契约

- `product.proto`：`ProductQueryService{GetProduct, GetProducts}`；decimal 用 **string** 承载（protobuf 无 decimal 类型，与 Java JSON 序列化 BigDecimal 语义兼容）。
- `payment.proto`：`PaymentCommandService{Pay, Refund}`；**Pay 幂等**（同一订单重复 Pay 返回同一支付单 ID —— RPC 重试不产生重复支付）。
- 重新生成（需要 protoc 31+ 与 protoc-gen-go/protoc-gen-go-grpc，装于 PATH）：

```bash
make proto
```

## 运行（零外部设施）

```bash
# 4 个终端，或 make run-ms（sh 后台方式）
make run-ms-product &   # REST 8083 + gRPC 9183
make run-ms-payment &   # REST 8082 + gRPC 9182
make run-ms-order &     # REST 8081
make run-ms-gateway     # 8080 统一入口
```

全链路体验（全部经网关 8080）：

```bash
curl -X POST http://localhost:8080/api/products -H "Content-Type: application/json" \
  -d '{"name":"Laptop","price":"999.99","stock":10}'

curl -X POST http://localhost:8080/api/orders -H "Content-Type: application/json" \
  -d '{"customerId":"customer-1","lines":[{"productId":"<productId>","quantity":2}]}'

curl -X POST http://localhost:8080/api/orders/<orderId>/pay
curl http://localhost:8080/api/orders/<orderId>          # PAID
curl -X POST http://localhost:8080/api/orders/<orderId>/cancel -H "Content-Type: application/json" -d '{"reason":"changed mind"}'
curl http://localhost:8080/api/payments                   # REFUNDED
```

错误契约（每服务自持 GlobalExceptionHandler，网关原样透传）：
`{"errorCode":"ORDER_NOT_FOUND","message":"..."}` → 404；业务规则/校验/fallback → 400。

## 测试（18 个 Java 测试的对应物，零外部设施）

```bash
go test ./...    # workspace 内逐模块：make test 也会跑
```

| Java 测试 | Go 对应 | 说明 |
|---|---|---|
| 8 个领域测试 | `product/domain` + `payment/domain` + `order/domain` 单测 | 状态机、幂等、校验 |
| 1 个流程测试（@MockBean 远程服务） | `order/application/impl` flow 测试 | fake 契约注入 + 真 GORM 仓储（SQLite 内存库）—— provider 契约让跨服务测试不需要运行中的进程 |
| 9 条 ArchUnit 规则 | `order/architecture_rules_test.go`（go/list 跨 5 模块扫描） | order 只准依赖 api+pb、RPC 客户端只在 infrastructure、层方向等 |
| —（Java 需要起全服才能测网关链路） | `gateway/e2e_test.go` | 进程内拉起 3 服务（真实 TCP gRPC）+ httptest 网关：全链路下单→支付→取消退款、双路熔断 fallback、404 透传 |
| — | `payment/interfaces/rpc` 契约测试 | 真实 gRPC server 直连：Pay 幂等、错误→status code 映射契约 |

## 演进点（Go 版当前简化、P5+ 接入）

| 能力 | 现状 | 演进 |
|---|---|---|
| 服务发现 | env 固定地址 | 接缝已留：替换 `ordergrpc.NewProductClient` 的地址来源即可（consul/etcd/DNS） |
| 负载均衡 | 单实例 | gRPC 自带 `pickfirst`；多实例随发现组件接入 `round_robin` |
| 链路追踪 | slog 结构化日志 | gRPC interceptor 注入 trace-id（OpenTelemetry） |
| 事件总线 | 进程内同步 EventBus | P5：Outbox + 消息代理（`EventPublisherService` 端口不变） |
| 分布式事务 | 编排式（cancel 退款） | P6：SAGA/TCC |
