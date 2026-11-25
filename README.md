# go-tutorials

Go 语言学习指南 —— 参照 [java-tutorials](../java-tutorials) 的模块结构，用 **Go 重写同样的架构实践**，每个 demo 都能独立运行，并与 Java 版一一对照。

## 学习路线（参照 java-tutorials/architecture）

| 阶段 | 模块 | 对应 Java 版 | 核心学习点 | 状态 |
|---|---|---|---|---|
| P0 | 仓库脚手架：go.work / Makefile / 统一依赖 | Maven 多模块 + parent POM | Go module 与 workspace 机制 | ✅ |
| P1 | `architecture/monolithic` | `architecture/monolithic` | DDD 四层、依赖倒置、手动 DI、领域事件、事务边界 | ✅ |
| P2 | `architecture/monolithic-multi-module` | `architecture/monolithic-multi-module` | go.work 多 module、shared-kernel 复用 | ✅ |
| P3 | `architecture/erp` | `architecture/erp` | 平台层（kernel/persistence/web/messaging）+ 业务模块 | ⬜ |
| P4 | `architecture/microservices` | `architecture/microservices` | gin 网关 + 多服务，gRPC + protobuf（对应 Feign） | ⬜ |
| P5 | `architecture/event-driven` | `architecture/event-driven` | Outbox、事件信封、幂等去重、Saga | ⬜ |
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
    └── monolithic-multi-module/     # P2：多模块单体（已完成，5 个 Go module）
        ├── shared/                  #   内核模块
        ├── product/                 #   产品上下文模块
        ├── payment/                 #   支付上下文模块
        ├── order/                   #   订单上下文模块
        └── bootstrap/               #   组装 + 入口 + 测试（SQLite 内存库，零依赖启动）
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

## 环境说明

- Go ≥ 1.26
- Docker Desktop（跑 MySQL）
- Go 代理：`go env -w GOPROXY=https://goproxy.cn,direct`（国内网络必备）
