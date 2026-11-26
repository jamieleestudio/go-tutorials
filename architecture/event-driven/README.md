# ⑤ 事件驱动架构 (Event-Driven Architecture — 务实版)

与 ④ microservices 的同步 RPC 形成**分布式两种范式对比**：异步事件 + **本地消息表（Outbox）** + **Saga 事件协同** + **最终一致性**。不做 CQRS/事件溯源（生产采用率低）。

## 与 ④ 的本质差异

| | ④ 同步 RPC | ⑤ 事件驱动 |
|---|---|---|
| 通信 | 同步 gRPC | **异步事件**（Outbox → 投递） |
| 一致性 | 即时 | **最终一致**（查状态接口兜底） |
| 耦合 | 运行时耦合（等响应） | **时间解耦**（发布者不等消费者） |
| 故障 | 调用链中断 | 消费者宕机不影响下单 |
| 消息可靠性 | — | **Outbox**（业务+消息同事务） |
| 重复消费 | orderId 幂等 | **eventId 去重表** |

## 投递介质：Java 用 Kafka，Go 教学版用 HTTP webhook

Java 版经 `@KafkaListener` / `KafkaTemplate` 收发事件。Go 教学版改用 **HTTP webhook**（Outbox Relay POST 到订阅者 `/internal/events`），**零外部设施即可跑通全部语义**：

| Kafka 语义 | webhook 版对应物 |
|---|---|
| at-least-once 投递 | Relay 投递失败不标记 published → 下轮重投 |
| 消费 ack | 处理成功返回 200；失败返回 5xx → 上游重投 |
| 重复消费安全 | `processed_events` 表按 eventId 去重（不变） |
| topic → 订阅 | `EventSubscribers map[topic][]endpoint` |

投递介质被抽象为 **`DeliverFunc` 接缝**（`outbox.HTTPDeliver(subscribers)` 是生产实现）——换 Kafka/NATS 只需提供新的 `DeliverFunc`，其余代码零改动。这是留作演进点的接缝。

务实的混合模式与 Java 版一致：**目录读走同步 gRPC**（order → product），**状态变更走异步事件**。

## Saga 流程（事件协同，无中心协调器）

```
POST /api/orders → order 本地事务: [保存订单 + Outbox写OrderCreatedEvent] → 立即返回(CREATED)

payment 消费 OrderCreatedEvent → 支付 → Outbox写PaymentSucceededEvent
product 消费 OrderCreatedEvent → 扣库存(预校验全部行) → 记账 → Outbox写InventoryDeductedEvent

order 消费两者:
  PaymentSucceeded  → order.markPaymentSucceeded()
  InventoryDeducted → order.markInventoryDeducted()
  两者都到 → 状态 CONFIRMED ✅（最终一致性时刻）

失败补偿:
  缺货/缺品 → product 发 InventoryDeductFailed → order.cancel → OrderCancelledEvent
  → payment 退款、product 凭 deducted_orders 记录精确回补
```

## 模块结构（4 个 Go module，对照 Java 5 个）

```
event-driven/
├── shared/    纯内核 + 集成事件契约（event 包：6 个事件 + Envelope + 类型常量）
├── product/   REST :8083 + gRPC :9183（目录读）+ /internal/events（扣减/回补）
│              └── deducted_orders 补偿记账（Saga 补偿凭据）
├── payment/   纯事件进程（无 REST 业务端点/gRPC）：/internal/events + Relay（:8082）
└── order/     REST :8081 + Saga 状态机 domain + Outbox + /internal/events（双事件汇合点）
```

Java 版的 `eda-product-api` 在 Go 版是 `product/api` 契约包（P4 同款模式：跨服务只认 api 包 + protobuf）。

## 关键机制

**本地消息表（Outbox）**：`outbox.OutboxEventPublisher`（实现应用层 `EventPublisherService` 端口）把事件写入 `outbox_messages` 表——与业务数据**同一本地事务**；`outbox.Relay`（默认 1s 轮询）异步投递。不丢单也不丢消息。

> **Go 与 Java 的事务载体差异**：Java 靠 `@Transactional` 线程绑定拿到事务句柄；Go 版没有这个魔法，**事务句柄就是 `context.Context`**（`shared.TxRunner` 把 gorm tx 放进 ctx，仓储用 `persistence.DBFromContext` 取出）。因此发布端口签名是 `Publish(ctx, evt)` 而非 `Publish(evt)`——签名错了 outbox 就落到事务外，原子性被静默破坏。

**消费幂等（双层）**：
1. `processed_events` 表按 eventId 去重（at-least-once 重投安全）——查重、标记与业务处理在**同一事务**；
2. 业务幂等兜底（如 payment 按 orderId 查已有支付单）——即使 relay 全量重放也不会重复处理。

**Order Saga 状态机**（`order/domain`）：`CREATED → (paymentSucceeded && inventoryDeducted) → CONFIRMED`，只有 `CREATED` 可 `CANCELLED`。双布尔进度列是一等公民（`payment_succeeded` / `inventory_deducted`），迟到/重放的事件被领域守卫拒绝。

**补偿记账**：product 扣减前先写 `deducted_orders`（一行一订单，行明细 JSON 列）——取消时凭记账精确回补并删除记录；没有记账就跳过（幂等）。

**零部分扣减**：product 预校验【全部】订单行后再统一扣减——缺货时零行被扣。业务失败被 catch：发布 `InventoryDeductFailed`（同事务提交，触发 Saga 补偿）；只有发布/标记本身的故障才回滚重试。

## 运行（零外部设施）

```bash
# 3 个终端分别启动（或 make run-eda）：
go run ./architecture/event-driven/product/cmd/productsvc   # REST :8083 + gRPC :9183
go run ./architecture/event-driven/payment/cmd/paymentsvc   # HTTP :8082（仅事件入口）
go run ./architecture/event-driven/order/cmd/ordersvc       # REST :8081

# 全链路体验（Saga 收敛默认 ~1s = Relay 轮询间隔）：
curl -X POST http://localhost:8083/api/products -H "Content-Type: application/json" \
  -d '{"name":"Laptop","price":"100.00","stock":10}'
curl -X POST http://localhost:8081/api/orders -H "Content-Type: application/json" \
  -d '{"customerId":"customer-1","lines":[{"productId":"<productId>","quantity":2}]}'
# 立即返回 CREATED；等 ~2s 后：
curl http://localhost:8081/api/orders/<orderId>   # → CONFIRMED + paymentSucceeded + inventoryDeducted
```

## 配置（环境变量）

| 变量 | 默认 | 说明 |
|---|---|---|
| `DB_DSN` | `sqlite://file:eda-<svc>?mode=memory&cache=shared` | 可切 `mysql://...` |
| `OUTBOX_RELAY_INTERVAL` | `1s` | Outbox 轮询间隔 |
| `ORDER_EVENTS_SUBSCRIBERS` | `http://localhost:8082/internal/events,http://localhost:8083/internal/events` | order-events 订阅者（逗号分隔） |
| `PAYMENT_EVENTS_SUBSCRIBERS` | `http://localhost:8081/internal/events` | payment-events 订阅者 |
| `PRODUCT_EVENTS_SUBSCRIBERS` | `http://localhost:8081/internal/events` | product-events 订阅者 |
| `PRODUCT_GRPC_ADDR` | `localhost:9183` | order 同步目录读的目标 |
| `*_HTTP_ADDR` / `PRODUCT_GRPC_ADDR` | 各服务默认端口 | 监听地址 |

## 测试

```bash
cd architecture/event-driven/order && go test ./...
cd architecture/event-driven/payment && go test ./...
```

- **e2e 4 场景（Go 增强）**：Java 版的 Saga 集成验证依赖 testcontainers/Kafka；Go 版因 webhook 投递，**进程内起真实 TCP 三服务栈**（真 gin + 真 gRPC + 真 SQLite + 真 Relay，随机端口动态接线）：
  1. 全链路 Saga → 双到齐 CONFIRMED（最终一致性）
  2. 缺货 → InventoryDeductFailed → 订单 CANCELLED（零部分扣减）
  3. 取消补偿 → 库存凭记账回补（3→2→3）
  4. 同一 eventId 重放 → processed_events 去重 → 只扣一次
- **domain 7 个**：Saga 状态机（对照 Java `OrderSagaTest`，双到齐顺序无关、终态守卫）
- **outbox 3 个**：Relay 成功标记/空表跳过/失败留 pending（对照 Java `OutboxRelayTest` 的 2 个 + Go 增强失败重投）
- **架构规则 6 条**：领域纯净（禁 gin/gorm/grpc/protobuf/encoding-json）、application 不依赖 infrastructure、interfaces 不依赖 infrastructure/impl、outbox 只属于 infrastructure、payment/product 不依赖 order、shared 根包纯净
