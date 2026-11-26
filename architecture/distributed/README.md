# ⑥ 分布式架构 (Distributed — 朴素同步 RPC 版)

微服务的第三种范式：**没有注册中心、没有 gRPC、没有熔断框架** —— 3 个独立进程用**最朴素的 HTTP** 互相同步调用（对应 Java 版 3 个 Spring Boot 进程 + `RestClient`）。刻意保持朴素，是为了把分布式调用最要命的三件事摊开在同一处看清楚：

- **幂等（idempotency）**：payment 按 orderId 幂等 —— 同一订单重复支付返回同一支付单 ID；
- **超时（timeout）**：连接 2s / 读 3s，不设超时的 RPC 是事故之源；
- **重试（retry）**：最多尝试 2 次，且**只重试网络层错误**（对应 Java 的 `ResourceAccessException`），拿到状态码的业务错误绝不重试。

三件事是一个闭环：网络层重试意味着同一个 POST 可能被执行两次 —— **客户端敢重试，靠的是服务端幂等**。

> Java 版的实际内容就是"同步 HTTP RPC + 幂等 + 超时 + 重试"，**没有** Seata/TCC（此前的"SAGA/TCC 分布式事务"描述不准确，已修正）。Saga/最终一致性见 ⑤。

## 与 ④ ⑤ 的范式对比

| | ④ 微服务 (gRPC) | ⑤ 事件驱动 | ⑥ 分布式（本篇） |
|---|---|---|---|
| 通信 | 同步 gRPC + protobuf | 异步事件（Outbox） | **同步纯 HTTP + JSON** |
| 服务发现 | 固定地址 + env | 订阅表 + env | **固定地址 + env（无注册中心）** |
| 容错 | gobreaker 熔断 | 时间解耦 | **超时 + 手写重试（≤2 次，仅网络层）** |
| 一致性 | 即时 | 最终一致 | 即时 |
| 重复调用防护 | orderId 幂等 | eventId 去重表 | **orderId 幂等（重试安全的根基）** |

④ 的客户端已经带了熔断/超时等工业件；⑥ 把它们全部拿掉，只留手写的 `Transport` 超时与重试循环 —— 看清工业件底下到底是什么。

## RPC 契约（provider-defined，跨服务只认 `*/api` 包）

| 端点 | 方法 | 请求 | 响应 | 语义 |
|---|---|---|---|---|
| `/rpc/payments/pay?orderId=&amount=` | POST | query 参数（金额 `StringFixed(2)`） | **纯文本 paymentId** | **幂等**：同单同 ID（对应 Java 返回 `String`） |
| `/rpc/payments/{paymentId}/refund` | POST | — | 200 空 | 退款（仅 SUCCESS 可退） |
| `/rpc/products/{productId}` | GET | — | `ProductDto` JSON | 未知商品 → 404 `PRODUCT_NOT_FOUND` |
| `/rpc/products?ids=a,b` | GET | — | `[]ProductDto` JSON | 不存在的 ID 不报错，只返回找到的部分 |

契约的 JSON 字段名由 `product/api`、`payment/api` 契约包里的 **json tag** 决定（对应 Java record 的 Jackson 序列化）。对外 REST（`/api/*`）与对内 RPC（`/rpc/*`）在同一进程并存，路径即边界。

## 模块结构（4 个 Go module，对照 Java 同构）

```
distributed/
├── shared/     内核 + persistence（DB 打开/事务传播）
├── product/    REST :8083 + RPC /rpc/products（提供方）
├── payment/    REST :8082 + RPC /rpc/payments（提供方，Pay 幂等）
└── order/      REST :8081（编排方）
    ├── infrastructure/rpc   超时 + 重试的 HTTP 客户端（对应 Java 的两个 RPC client）
    ├── infrastructure/event 进程内事件（对应 Spring @EventListener，仅打日志）
    └── app                  组合根：REST + 2 个远程契约 + EventBus
```

order 是唯一的编排者：`CreateOrder` 远程查品 → `PayOrder` 远程支付 → `CancelOrder` 远程退款，领域事件全部同进程（P6 无集成事件）。

## 三件套的代码落点

**超时 + 重试（`order/infrastructure/rpc/clients.go`）**：

```go
Transport: &http.Transport{
    DialContext:           (&net.Dialer{Timeout: 2 * time.Second}).DialContext,  // 连接 2s
    ResponseHeaderTimeout: 3 * time.Second,                                      // 读 3s
}
// doWithRetry: client.Do 出错（网络层）→ 重试，最多 maxAttempts=2 次；
// checkStatus: 拿到 HTTP 状态码 → 业务错误 → 绝不重试
```

**幂等（`payment/application/impl/payment_service_impl.go`）**：

```go
existing, _ := s.paymentRepository.FindByOrderID(ctx, orderID)
if existing != nil {
    return existing.ID(), nil   // 幂等：同一订单重复支付 → 同一支付单 ID
}
```

**架构守护（`order/architecture_rules_test.go`，对照 Java ArchUnit 的 9 条）**：order 与 payment/product 之间**只准经过 api 契约包**；RPC 客户端只住在 `order/infrastructure/rpc`（application/domain/interfaces 永不感知 HTTP 细节）。

## 运行（零外部设施）

```bash
# 3 个终端分别启动（或 make run-dist）：
go run ./architecture/distributed/product/cmd/productsvc   # REST + RPC :8083
go run ./architecture/distributed/payment/cmd/paymentsvc   # REST + RPC :8082
go run ./architecture/distributed/order/cmd/ordersvc       # REST :8081

# 全链路体验（order → payment/product 是真实跨进程 HTTP 调用）：
curl -X POST http://localhost:8083/api/products -H "Content-Type: application/json" \
  -d '{"name":"Laptop","price":"999.99","stock":10}'
curl -X POST http://localhost:8081/api/orders -H "Content-Type: application/json" \
  -d '{"customerId":"customer-1","lines":[{"productId":"<productId>","quantity":2}]}'
curl -X POST http://localhost:8081/api/orders/<orderId>/pay          # → PAID（远程支付）
curl http://localhost:8081/api/orders/<orderId>

# 幂等体验：对 payment 服务重放同一笔 pay RPC → 返回同一 paymentId，不产生第二笔支付
curl -X POST "http://localhost:8082/rpc/payments/pay?orderId=<orderId>&amount=1999.98"

curl -X POST http://localhost:8081/api/orders/<orderId>/cancel -H "Content-Type: application/json" -d '{"reason":"changed mind"}'
# → 订单 CANCELLED + payment REFUNDED（远程退款）
```

## 配置（环境变量）

| 变量 | 默认 | 说明 |
|---|---|---|
| `DB_DSN` | `sqlite://file:dist-<svc>?mode=memory&cache=shared` | 可切 MySQL DSN |
| `PAYMENT_SERVICE_URL` | `http://127.0.0.1:8082` | order 的 payment RPC 目标（对应 Java application.yml 固定地址） |
| `PRODUCT_SERVICE_URL` | `http://127.0.0.1:8083` | order 的 product RPC 目标 |
| `*_HTTP_ADDR` | `:8081` / `:8082` / `:8083` | 各服务监听地址 |

## 测试

```bash
cd architecture/distributed/order   && go test ./...
cd architecture/distributed/payment && go test ./...
cd architecture/distributed/product && go test ./...
```

- **RPC 客户端 5 个（Go 增强）**：网络层错误重试到成功（断言恰好 2 次请求）、业务错误绝不重试（断言恰好 1 次）、对端挂起在读超时处失败、refund 路径映射与 404 透传、product 契约 JSON 往返 —— Java 版的 RpcClient 逻辑没有专门单测，Go 版把它变成可执行的规约。
- **e2e 4 场景（Go 增强）**：进程内起**真实 TCP 三服务栈**（真 gin + 真 SQLite + 真随机端口 RPC 接线）：
  1. 全链路 pay → 订单 PAID + payment SUCCESS（远程查品 + 远程支付两次跨进程生效）
  2. **幂等重放**：对 payment 服务重放同一笔 pay RPC → 同一 paymentId、支付单总数仍为 1；订单层面重复支付 → 400 规则违约
  3. 取消退款：cancel → 订单 CANCELLED + payment REFUNDED
  4. product RPC 契约：单查/批查（缺品不报错）/404 错误契约
- **flow 2 个**（对应 Java `OrderServiceFlowTest`）：@MockBean 的等价物是 fake —— provider-defined 接口让跨服务测试不需要真实进程；仓储用真 SQLite 内存库，编排逻辑全走真实代码。
- **domain 14 个**：order 7（对照 Java `OrderTest` 的 8 用例，状态矩阵覆盖含 SHIPPED 转换）+ payment 4 + product 3。
- **架构规则 11 条**：Java 9 条 1:1 映射（order 只碰 `*/api` 契约、RPC 客户端只住 order infrastructure、product/payment 互不依赖、分层方向……）+ Go 附加的契约包纯净与 shared 根包纯净。

## 与 Java 版的差异

| 差异点 | Java | Go |
|---|---|---|
| 金额序列化 | `BigDecimal` 经 query 传递，Jackson 保留 scale | `StringFixed(2)` 走 query（money 边界）；`decimal.Decimal` 的 JSON 序列化裁尾零（`"42.00"` → `"42"`） |
| 测试 | 3 个 Spring Boot 进程手工联调 | 进程内真实 TCP 三服务栈 + fake 注入，`go test` 全覆盖 |
| 事件监听 | `interfaces/event` 的 `@EventListener` | 进程内 EventBus，`infrastructure/event`（仅打日志，P6 无集成事件） |
