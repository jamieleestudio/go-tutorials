# architecture/monolithic —— 单体四层架构（DDD）

对 Java 版 `java-tutorials/architecture/monolithic` 的 Go 重写：**同一个业务**（order / product / payment 三个限界上下文，下单 → 支付 → 取消退款），**同一套分层规则**，用 Go 的方式表达。

## 架构总览

```
                        gin HTTP 引擎
             ┌──────────────────────────────┐
             │  Logger / Recovery /         │
             │  ErrorHandler(错误→HTTP映射)  │   ← 对应 @RestControllerAdvice
             └──────┬──────────┬─────────┬──┘
                    │          │         │
        ┌───────────▼──┐ ┌─────▼──────┐ ┌▼────────────┐
        │ order        │ │ product    │ │ payment     │      interfaces 层
        │  (web/event) │ │  (web)     │ │  (web)      │
        └───────┬──────┘ └─────┬──────┘ └──────┬──────┘
                │              │               │      application 层
        ┌───────▼──────┐ ┌─────▼──────┐ ┌──────▼─────┐      （Service 接口+impl）
        │ application  │ │ application│ │ application│◄───── order 只依赖
        │  + impl      │ │  + impl    │ │  + impl    │      另两家的【接口】
        └───────┬──────┘ └─────┬──────┘ └──────┬──────┘
                │              │               │      domain 层
        ┌───────▼──────┐ ┌─────▼──────┐ ┌──────▼─────┐      （聚合根/值对象/
        │ domain       │ │ domain     │ │ domain     │       状态机/仓储端口）
        │ + domain/event│ │            │ │            │
        └───────┬──────┘ └─────┬──────┘ └──────┬──────┘
                │              │               │      infrastructure 层
        ┌───────▼──────────────▼───────────────▼─────┐      （GORM 实体、
        │  infrastructure: 仓储实现 + 事件发布实现      │       依赖倒置的实现侧）
        └───────────────────┬────────────────────────┘
                            │
        ┌───────────────────▼───────────────┐
        │ platform: GORM 连接/TxRunner/中间件 │      横切基础设施
        └───────────────────┬───────────────┘
                            │
                        MySQL 8 (Docker)

        shared 内核（errors/AggregateRoot/DomainEvent/EventBus/ID/TxRunner 端口）
        被所有上下文依赖，但【不依赖任何上下文】
```

## 目录结构（Java → Go 对照）

| Java 包 | Go 包 | 内容 |
|---|---|---|
| `com.example.monolithic.order.domain` | `internal/order/domain` | Order 聚合根、OrderItem 值对象、OrderStatus 状态机、OrderRepository 端口 |
| `...order.domain.event` | `internal/order/domain/event` | OrderCreated/Paid/CancelledEvent |
| `...order.application(.impl)` | `internal/order/application(/impl)` | OrderService 接口、Command/Query、DTO、实现 |
| `...order.interfaces.web` | `internal/order/interfaces/web` | OrderController（gin） |
| `...order.interfaces.event` | `internal/order/interfaces/event` | 事件监听器 |
| `...order.infrastructure` | `internal/order/infrastructure` | OrderEntity(GORM)、仓储实现、事件发布实现 |
| `...shared` | `internal/shared` | 领域错误、聚合根基类、事件总线、ID、TxRunner 端口 |
| Spring Boot 自动配置 | `internal/platform` | 配置、GORM 连接、事务传播、全局错误中间件 |
| `MonolithicApplication` | `cmd/monolithic` + `internal/app` | 入口 + 组合根（手动 DI） |

## 运行

```powershell
# 启动 MySQL（端口 13306，库名 monolith；集成测试用 monolith_test）
docker compose up -d

# 启动应用（自动建表）
go run ./cmd/monolithic

# 测试（无 MySQL 时集成测试自动跳过）
go test ./...
```

配置通过环境变量覆盖（对应 application.yml）：`APP_PORT`（默认 8080）、`DB_DSN`、`GORM_LOG_LEVEL`（silent/error/warn/info）。

## API 一览（与 Java 版完全一致）

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/products` | 创建产品 `{name, price, stock}` → 201 |
| GET | `/api/products` | 产品列表 |
| GET | `/api/products/{id}` | 产品详情（404 → `{"errorCode":"PRODUCT_NOT_FOUND"}`） |
| POST | `/api/orders` | 下单 `{customerId, lines:[{productId, quantity}]}` → 201 |
| POST | `/api/orders/{id}/pay` | 支付（联动 Payment 上下文，同一事务） |
| POST | `/api/orders/{id}/cancel` | 取消（已支付则自动退款） |
| GET | `/api/orders/{id}` | 订单详情 |
| GET | `/api/orders?customerId=` | 按客户查询（缺参 → 400 VALIDATION_ERROR） |
| POST | `/api/payments` | 创建支付单 `{orderId, amount}` |
| POST | `/api/payments/{id}/process` | 处理支付（demo 中必然成功） |
| GET | `/api/payments` / `/api/payments/{id}` | 支付查询 |

## 关键学习点（Java → Go 映射）

| Java | Go | 说明 |
|---|---|---|
| `@SpringBootApplication` + 组件扫描 | `internal/app/app.go` 组合根 | 没有容器，依赖关系全部手工装配，依赖图一目了然 |
| `@Transactional`（AOP 隐式） | `TxRunner.WithinTx(ctx, fn)`（显式） | 事务句柄通过 **context 向下传播**；`REQUIRED` 传播 = ctx 已有事务直接加入 |
| `Optional<T>` 仓储返回值 | `(T, error)` + 哨兵错误 | `ErrOrderNotFound` 由仓储返回，应用层转换为 `EntityNotFound` |
| 异常继承体系 | 单一 `*shared.DomainError` + `Kind` 字段 | `errors.As` 断言类型，中间件统一映射 HTTP 码 |
| `record` | 普通 struct | Command / Query / DTO / 领域事件都是 struct |
| `BigDecimal` | `shopspring/decimal` | 永远不要用 float 存钱 |
| `@EventListener` 反射分发 | EventBus 按 `EventName()` 订阅 + 类型断言 | 全部是明式代码，无反射魔法 |
| Lombok | 不需要 | Go 没有注解处理器，也没有这个需求 |
| ArchUnit 字节码规则 | `go list -json` + 自写 import 规则测试 | 见下"架构守护规则" |
| `application.yml` | 环境变量 + 默认值 | 12-factor 风格 |

## 架构守护规则（`architecture_rules_test.go`，对应 ArchUnit）

| # | 规则 | Java 版对应用例 |
|---|---|---|
| 1 | domain 不依赖 application/infrastructure/interfaces，第三方只允许 decimal/uuid/shared | `domain_layers_do_not_depend_on_*` |
| 2 | interfaces 不访问 infrastructure，只注入 Service 接口不碰 impl | `interfaces_layer_does_not_access_*` |
| 3 | infrastructure / impl 不访问 interfaces | `*_does_not_access_interfaces` |
| 4 | infrastructure 不跨上下文访问 | `infrastructure_layers_do_not_cross_contexts` |
| 5 | order 只依赖 product/payment 的 application 接口层 | `order_context_does_not_access_*` |
| 6 | product/payment 不依赖 order | `product_and_payment_do_not_depend_on_order` |
| 7 | product 与 payment 相互独立 | `product_does_not_depend_on_payment_*` |
| 8 | shared 内核不依赖任何业务上下文 | `shared_kernel_does_not_depend_on_*` |

运行方式就是普通测试：`go test -run "Layer\|Context\|Purity" ./...`。破坏分层 = 测试变红。

## 事务边界说明

- `CreateOrder` / `PayOrder` / `CancelOrder` 通过 `TxRunner` 开启事务：**订单落库、（支付/退款）、事件发布在同一个 MySQL 事务里**；
- `REQUIRED` 传播：Order 服务开启的事务会传给下游 Payment 服务与各仓储（都从 ctx 取事务句柄），与 Spring 行为一致；
- 生产建议把 AutoMigrate 换成 golang-migrate 等迁移工具（对应 Flyway/Liquibase）。
