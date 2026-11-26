# go-tutorials

Go 语言学习指南 —— 参照 [java-tutorials](../java-tutorials) 的模块结构，用 **Go 重写同样的架构实践**，每个 demo 都能独立运行，并与 Java 版一一对照。

## 学习路线（参照 java-tutorials/architecture）

| 阶段 | 模块 | 对应 Java 版 | 核心学习点 | 状态 |
|---|---|---|---|---|
| P0 | 仓库脚手架：go.work / Makefile / 统一依赖 | Maven 多模块 + parent POM | Go module 与 workspace 机制 | ✅ |
| P1 | `architecture/monolithic` | `architecture/monolithic` | DDD 四层、依赖倒置、手动 DI、领域事件、事务边界 | ✅ |
| P2 | `architecture/monolithic-multi-module` | `architecture/monolithic-multi-module` | go.work 多 module、shared-kernel 复用 | ✅ |
| P3 | `architecture/erp` | `architecture/erp` | 平台层 + api 契约层（域间只走 api）+ 端分包 + 多端装配 | ✅ |
| P4 | `architecture/microservices` | `architecture/microservices` | gin 网关 + 多服务，gRPC + protobuf（对应 Feign） | ✅ |
| P5 | `architecture/event-driven` | `architecture/event-driven` | Outbox、事件信封、幂等去重、Saga | ✅ |
| P6 | `architecture/distributed` | `architecture/distributed` | 分布式事务（SAGA/TCC）、分布式 ID | ⬜ |
| P7 | `architecture/cloud-native` | `architecture/cloud-native` | 健康探针、优雅停机、Dockerfile、K8s | ⬜ |
| P8 | `architecture/evolvable-architecture` | 同名文档模块 | 演进式架构笔记（中文） | ⬜ |

## 统一技术选型

| 维度 | 选型 | Java 对照 |
|---|---|---|
| Web | gin | Spring MVC |
| RPC | gRPC + protobuf（P4 起） | OpenFeign |
| 数据库 | MySQL 8（docker-compose）+ GORM | MySQL + JPA/Hibernate |
| 金额 | shopspring/decimal | BigDecimal |
| 事件 | 进程内 EventBus（channel 升级） | ApplicationEventPublisher |
| 事务 | TxRunner 端口 + context 传播 | @Transactional |
| DI | 构造函数手动注入（组合根） | Spring IoC |
| 测试 | go test + testify | JUnit 5 + AssertJ |
| 架构守护 | `go list` 解析 import 的自写规则测试 | ArchUnit |

## 目录地图

```
go-tutorials/
├── go.work                  # workspace：聚合所有 Go module
├── Makefile                 # make test / make run / make compose-up
└── architecture/
    ├── monolithic/                  # P1：单体四层架构（已完成）
    ├── monolithic-multi-module/     # P2：多模块单体（已完成，5 个 Go module）
    │   ├── shared/                  #   内核模块
    │   ├── product/                 #   产品上下文模块
    │   ├── payment/                 #   支付上下文模块
    │   ├── order/                   #   订单上下文模块
    │   └── bootstrap/               #   组装 + 入口 + 测试（SQLite 内存库，零依赖启动）
    └── erp/                         # P3：ERP 可演进架构（已完成，单 module 包优先）
        ├── shared/                  #   共享内核（DomainError/AggregateRoot）
        ├── platform/                #   平台层：kernel/persistence/security/web/messaging/integration
        ├── system/                  #   纯提供方域（api 契约 + provider）
        ├── grade/                   #   被跨域调用的域（五层 + web）
        ├── attendance/              #   完整样例：五层 + 8 类端 + 领域事件 + 人脸识别端口
        └── app/                     #   组合根（EndSet 多端装配）+ 4 个部署形态入口 + 测试
    ├── microservices/               # P4：微服务（已完成，5 个 Go module，gRPC + protobuf）
    │   ├── shared/                  #   共享内核 + persistence（DB 打开/事务传播）
    │   ├── product/                 #   商品服务：REST :8083 + gRPC :9183（提供方）
    │   ├── payment/                 #   支付服务：REST :8082 + gRPC :9182（Pay 幂等，提供方）
    │   ├── order/                   #   订单服务：REST :8081，gRPC 客户端 + gobreaker 熔断（消费方）
    │   └── gateway/                 #   API 网关：:8080 统一入口（ReverseProxy）
    └── event-driven/                # P5：事件驱动（已完成，4 个 Go module，Outbox + Saga）
        ├── shared/                  #   内核 + 集成事件契约（6 事件 + Envelope）
        ├── product/                 #   商品服务：扣减/回补 + 补偿记账（REST :8083 + gRPC :9183）
        ├── payment/                 #   支付服务：纯事件进程（:8082 仅事件入口）
        └── order/                   #   订单服务：Saga 状态机 + 双事件汇合（REST :8081）
```

## 快速开始（monolithic）

```powershell
# 1. 启动 MySQL（需要先启动 Docker Desktop）
docker compose -f architecture/monolithic/docker-compose.yml up -d

# 2. 启动应用（自动建表）
go run ./architecture/monolithic/cmd/monolithic

# 3. 体验完整流程
curl -X POST http://localhost:8080/api/products -H "Content-Type: application/json" `
  -d '{"name":"Laptop","price":"999.99","stock":10}'

curl -X POST http://localhost:8080/api/orders -H "Content-Type: application/json" `
  -d '{"customerId":"customer-1","lines":[{"productId":"<上面的productId>","quantity":2}]}'

# 支付 / 查询 / 取消
curl -X POST http://localhost:8080/api/orders/<orderId>/pay
curl http://localhost:8080/api/orders/<orderId>
curl -X POST http://localhost:8080/api/orders/<orderId>/cancel -H "Content-Type: application/json" -d '{"reason":"changed mind"}'
```

测试（无 MySQL 时集成测试自动跳过）：

```powershell
go test ./...
```

详细说明见 [architecture/monolithic/README.md](architecture/monolithic/README.md)。

## 快速开始（P4 microservices，零外部设施）

```powershell
# 4 个终端分别启动（或 make run-ms）：
go run ./architecture/microservices/product/cmd/productsvc   # REST :8083 + gRPC :9183
go run ./architecture/microservices/payment/cmd/paymentsvc   # REST :8082 + gRPC :9182
go run ./architecture/microservices/order/cmd/ordersvc       # REST :8081
go run ./architecture/microservices/gateway/cmd/gateway      # :8080 统一入口

# 全链路体验（全部经网关 8080；order→product/payment 走 gRPC + gobreaker 熔断）
curl -X POST http://localhost:8080/api/products -H "Content-Type: application/json" `
  -d '{"name":"Laptop","price":"999.99","stock":10}'
curl -X POST http://localhost:8080/api/orders -H "Content-Type: application/json" `
  -d '{"customerId":"customer-1","lines":[{"productId":"<productId>","quantity":2}]}'
curl -X POST http://localhost:8080/api/orders/<orderId>/pay
curl http://localhost:8080/api/orders/<orderId>
curl -X POST http://localhost:8080/api/orders/<orderId>/cancel -H "Content-Type: application/json" -d '{"reason":"changed mind"}'
```

重新生成 protobuf 契约（需要 protoc + protoc-gen-go/-go-grpc 插件）：`make proto`。

详细说明见 [architecture/microservices/README.md](architecture/microservices/README.md)。

## 快速开始（P5 event-driven，零外部设施）

```powershell
# 3 个终端分别启动（或 make run-eda）：
go run ./architecture/event-driven/product/cmd/productsvc   # REST :8083 + gRPC :9183
go run ./architecture/event-driven/payment/cmd/paymentsvc   # HTTP :8082（仅事件入口）
go run ./architecture/event-driven/order/cmd/ordersvc       # REST :8081

# 全链路 Saga（下单立即返回 CREATED；~1-2s 后双事件到齐 CONFIRMED）：
curl -X POST http://localhost:8083/api/products -H "Content-Type: application/json" `
  -d '{"name":"Laptop","price":"100.00","stock":10}'
curl -X POST http://localhost:8081/api/orders -H "Content-Type: application/json" `
  -d '{"customerId":"customer-1","lines":[{"productId":"<productId>","quantity":2}]}'
curl http://localhost:8081/api/orders/<orderId>   # → CONFIRMED + paymentSucceeded + inventoryDeducted
curl -X POST http://localhost:8081/api/orders/<orderId>/cancel -H "Content-Type: application/json" -d '{"reason":"changed mind"}'
# 取消仅对 CREATED 有效；已扣库存凭 product 的补偿记账精确回补
```

Java 版经 Kafka 收发事件；Go 教学版用 HTTP webhook（Outbox Relay POST）实现同样的 at-least-once + 幂等去重语义，零外部设施。详见 [architecture/event-driven/README.md](architecture/event-driven/README.md)。

## 环境说明

- Go ≥ 1.26
- Docker Desktop（跑 MySQL）
- Go 代理：`go env -w GOPROXY=https://goproxy.cn,direct`（国内网络必备）
