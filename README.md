# go-tutorials

Go 语言学习指南 —— 参照 [java-tutorials](../java-tutorials) 的模块结构，用 **Go 重写同样的架构实践**，每个 demo 都能独立运行，并与 Java 版一一对照。

## 学习路线（参照 java-tutorials/architecture）

| 阶段 | 模块 | 对应 Java 版 | 核心学习点 | 状态 |
|---|---|---|---|---|
| 基础 | `base` | `base`（base/se） | Go 语法全集：类型/切片/map/接口/泛型/嵌套类、goroutine/channel/sync/context、同名算法重写 | ✅ |
| 专题 | `concurrent` | `concurrent`（j.u.c. 七个子模块） | synchronized↔Mutex、volatile↔atomic、Executor↔worker pool、Future/CF↔channel/errgroup、j.u.c. 工具箱、virtual-thread↔goroutine | ✅ |
| 专题 | `distributed-infra` | `distributed-infra`（distlock/outbox 已实现，id/transaction 空壳 pom） | 分布式锁「同一接口×3 介质」失效模式对照（残留/TTL/会话）、Outbox 完整版（SKIP LOCKED+退避+死信）、雪花 ID 落地 | ✅ |
| P0 | 仓库脚手架：go.work / Makefile / 统一依赖 | Maven 多模块 + parent POM | Go module 与 workspace 机制 | ✅ |
| P1 | `architecture/monolithic` | `architecture/monolithic` | DDD 四层、依赖倒置、手动 DI、领域事件、事务边界 | ✅ |
| P2 | `architecture/monolithic-multi-module` | `architecture/monolithic-multi-module` | go.work 多 module、shared-kernel 复用 | ✅ |
| P3 | `architecture/erp` | `architecture/erp` | 平台层 + api 契约层（域间只走 api）+ 端分包 + 多端装配 | ✅ |
| P4 | `architecture/microservices` | `architecture/microservices` | gin 网关 + 多服务，gRPC + protobuf（对应 Feign） | ✅ |
| P5 | `architecture/event-driven` | `architecture/event-driven` | Outbox、事件信封、幂等去重、Saga | ✅ |
| P6 | `architecture/distributed` | `architecture/distributed` | 朴素 HTTP RPC：幂等（orderId 同单同 ID）、超时、重试 | ✅ |
| P7 | `architecture/cloud-native` | `architecture/cloud-native` | 健康探针、优雅停机、Dockerfile、K8s/Helm | ✅ |
| P8 | `architecture/evolvable-architecture` | 同名文档模块 | 演进式架构笔记（中文） | ⬜ |

## 统一技术选型

| 维度 | 选型 | Java 对照 |
|---|---|---|
| Web | gin | Spring MVC |
| RPC | gRPC + protobuf（P4/P5）；P6 刻意朴素 HTTP + 手写超时/重试 | OpenFeign / RestClient |
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
├── base/                    # 语法基础（已完成，1 个 Go module，纯标准库）
│   ├── foundation/          #   类型/零值/iota 枚举/控制流/函数/错误处理（Java↔Go 对照）
│   ├── structure/           #   切片陷阱/map/struct 接收者/嵌入/隐式接口/泛型/嵌套类对应物
│   ├── concurrent/          #   goroutine/channel 全家桶/sync 原语/context（-race 验证）
│   ├── algorithm/           #   QuickSort 三 partition/树形菜单排序/队列实现栈/跳台阶（同名重写）
│   └── utils/               #   time 布局与时区坑/fmt 动词/strconv/strings/regexp
├── concurrent/              # 并发进阶专题（已完成，1 个 Go module，纯标准库，对照 j.u.c.）
│   ├── mutex/               #   ← sychronized：锁语义/DCL 四步演进→sync.Once/值复制陷阱（vet copylocks）
│   ├── volatile/            #   ← volatile：三句话↔atomic、快照发布、race detector 防线（构建标签拆分）
│   ├── executor/            #   ← executor：worker pool/拒绝策略/Ticker 丢 tick/优雅关闭三步曲
│   ├── future/              #   ← future+callable：channel 即 Future/池饥饿死锁/手写 errgroup
│   ├── tools/               #   ← tools：Latch/Barrier(Cond)/Semaphore/Exchanger/毒丸/RWMutex
│   └── virtthread/          #   ← virtual-thread：per-task goroutine 性能实验/ScopedValue↔context
├── distributed-infra/         # 分布式基础设施专题（已完成，1 个 Go module，对照 distributed-infra）
│   ├── distid/                #   ← distributed-id（Java 空壳 pom）：雪花算法落地（41/10/12 bit、时钟回拨拒绝）
│   ├── distlock/              #   ← distributed-lock：同一 DistLock 接口 × 3 介质失效模式对照
│   │   ├── mysql/             #     唯一索引锁 + FOR UPDATE 悲观锁（崩溃残留卡死；真 MySQL 不可达 skip）
│   │   ├── redis/             #     SET NX PX + Lua compare-and-del（TTL 过期自愈；miniredis 内嵌零设施）
│   │   └── zookeeper/         #     手写临时顺序节点 + watch 前驱（会话自愈；Curator 黑盒展开）
│   └── outbox/                #   ← outbox 完整语义版：SKIP LOCKED 认领 + 指数退避 + FAILED 死信
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
    └── distributed/                 # P6：分布式（已完成，4 个 Go module，朴素 HTTP RPC）
        ├── shared/                  #   内核 + persistence（DB 打开/事务传播）
        ├── product/                 #   商品服务：REST :8083 + RPC /rpc/products（提供方）
        ├── payment/                 #   支付服务：REST :8082 + RPC /rpc/payments（Pay 按 orderId 幂等）
        └── order/                   #   订单服务：REST :8081，手写超时+重试的 HTTP 客户端（消费方）
    └── cloud-native/                # P7：云原生（已完成，4 个 Go module + deploy，业务代码沿用 P6）
        ├── shared/                  #   内核 + persistence + cloudkit（探针/指标/优雅停机/12-Factor 配置）
        ├── product/                 #   商品服务：REST :8080（统一端口）+ RPC /rpc/products
        ├── payment/                 #   支付服务：REST :8080 + RPC /rpc/payments
        ├── order/                   #   订单服务：REST :8080，RPC 基址默认 K8s Service DNS
        └── deploy/                  #   docker 3×多阶段 Dockerfile + k8s 7×manifests + helm chart
```

## 快速开始（语法基础 base，零依赖）

每个主题都是「带 Java↔Go 对照注释的源文件 + 测试断言」，测试即文档：

```powershell
go test ./base/...                 # 全部语法主题
go test -race ./base/concurrent    # 并发主题开 -race 抓数据竞争
go test ./base/algorithm -v        # 快排三 partition / 菜单树排序 / 跳台阶
```

阅读顺序：`foundation` → `structure` → `concurrent` → `algorithm` → `utils`，
源文件顶部注释逐条标注 Java 概念的 Go 写法。详见 [base/README.md](base/README.md)。

## 快速开始（并发进阶 concurrent，零依赖）

Java j.u.c. 七个子模块的 Go 对照：坑演示全部**通道编排确定性复现**（非概率），
测试即文档：

```powershell
go test ./concurrent/...               # 全部并发专题（破损演示真跑：丢更新/重复创建真实发生）
go test -race ./concurrent/...         # race 构建：Java 没有的防线，修复版必须零竞争
go test ./concurrent/future -v -run TestPoolStarvation   # 池饥饿死锁：2 worker 死锁 vs 4 worker 通过
go test ./concurrent/virtthread -v -run TestPerformance  # Java 1 万任务 100s vs 1s 的 Go 版缩影
```

阅读顺序：`mutex` → `volatile` → `executor` → `future` → `tools` → `virtthread`。
「破损演示」按构建标签拆分：不带 `-race` 看坑真实发生，带 `-race` 证明修复版干净
—— race detector 正是 Java 没有的免费防线。详见 [concurrent/README.md](concurrent/README.md)。

## 快速开始（分布式基础设施 distributed-infra，需 Docker MySQL）

```powershell
# 1. 启动基础设施（MySQL 必须；ZK/Redis 供锁演示，测试探测不可达自动跳过）
docker compose -f distributed-infra/docker-compose.yml up -d

# 2. 跑测试（无基础设施时集成测试自动 skip，纯单测照常全绿）
go test ./distributed-infra/...
go test -race ./distributed-infra/...

# 3. Outbox 三场景 demo（同事务发布 / 回滚无泄漏 / 指数退避重试，约 20s）
go run ./distributed-infra/outbox/demo
```

教学主轴：分布式锁「同一接口 × 三种介质」的失效模式对照（MySQL 残留卡死 / Redis TTL
过期自愈 / ZK 会话自愈）；Java 空壳 pom 里排第一的 TODO（雪花算法）直接落地；
Outbox 完整语义版是 P5 简化版的深化（SKIP LOCKED 并发认领实证 + 指数退避 + FAILED 死信）。
详见 [distributed-infra/README.md](distributed-infra/README.md)。

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

## 快速开始（P6 distributed，零外部设施）

```powershell
# 3 个终端分别启动（或 make run-dist）：
go run ./architecture/distributed/product/cmd/productsvc   # REST :8083 + RPC /rpc/products
go run ./architecture/distributed/payment/cmd/paymentsvc   # REST :8082 + RPC /rpc/payments
go run ./architecture/distributed/order/cmd/ordersvc       # REST :8081

# 全链路（order → payment/product 是真实跨进程 HTTP RPC）：
curl -X POST http://localhost:8083/api/products -H "Content-Type: application/json" `
  -d '{"name":"Laptop","price":"999.99","stock":10}'
curl -X POST http://localhost:8081/api/orders -H "Content-Type: application/json" `
  -d '{"customerId":"customer-1","lines":[{"productId":"<productId>","quantity":2}]}'
curl -X POST http://localhost:8081/api/orders/<orderId>/pay          # → PAID（远程支付）
curl http://localhost:8081/api/orders/<orderId>

# 幂等体验：重放同一笔 pay RPC → 同一 paymentId，不产生第二笔支付
curl -X POST "http://localhost:8082/rpc/payments/pay?orderId=<orderId>&amount=1999.98"

curl -X POST http://localhost:8081/api/orders/<orderId>/cancel -H "Content-Type: application/json" -d '{"reason":"changed mind"}'
# → 订单 CANCELLED + payment REFUNDED（远程退款）
```

P6 刻意不引入 gRPC/注册中心/熔断框架：连接 2s / 读 3s 超时 + 最多 2 次的网络层重试全部手写在 `order/infrastructure/rpc`，客户端敢重试靠的是 payment 按 orderId 幂等（同单同 ID）。详见 [architecture/distributed/README.md](architecture/distributed/README.md)。

## 快速开始（P7 cloud-native，零外部设施）

业务代码与 P6 逐字节相同，只换云原生运维外壳：统一 :8080（`PORT` env 可覆盖）、
健康探针 `/healthz/liveness|readiness`、指标 `/metrics`、SIGTERM 优雅停机。

```powershell
# 3 个终端分别启动（或 make run-cn）：
# 容器里三服务都监听 8080，本地用 PORT 错开；order 的 RPC 地址指回 localhost
# —— 同一份代码不同 env，就是 12-Factor「配置外置」的演示
$env:PORT="8082"; go run ./architecture/cloud-native/payment/cmd/paymentsvc
$env:PORT="8083"; go run ./architecture/cloud-native/product/cmd/productsvc
$env:PAYMENT_SERVICE_URL="http://127.0.0.1:8082"; $env:PRODUCT_SERVICE_URL="http://127.0.0.1:8083"; go run ./architecture/cloud-native/order/cmd/ordersvc

# 云原生端点（每个服务都有；K8s 探针就打这两个路径）
curl http://127.0.0.1:8082/healthz/liveness    # {"status":"UP"}（恒 UP，不查依赖）
curl http://127.0.0.1:8082/healthz/readiness   # 未就绪 503 OUT_OF_SERVICE → 就绪 200 UP
curl http://127.0.0.1:8082/metrics             # Prometheus 文本（http_requests_total + go_*/process_*）
curl http://127.0.0.1:8082/info                # 服务名/版本/Go 版本/uptime

# 全链路（order → payment/product 是真实跨进程 HTTP RPC，与 P6 相同）：
curl -X POST http://127.0.0.1:8083/api/products -H "Content-Type: application/json" `
  -d '{"name":"Laptop","price":"999.99","stock":10}'
curl -X POST http://127.0.0.1:8081/api/orders -H "Content-Type: application/json" `
  -d '{"customerId":"customer-1","lines":[{"productId":"<productId>","quantity":2}]}'
curl -X POST http://127.0.0.1:8081/api/orders/<orderId>/pay          # → PAID（远程支付）
```

部署侧：`deploy/docker` 3 个多阶段 Dockerfile（distroless 非 root 镜像）、`deploy/k8s`
原生 manifests（探针/HPA/Ingress/ConfigMap/Secret + **preStop/terminationGracePeriodSeconds
优雅停机闭环，Java 版缺失的增强项**）、`deploy/helm/ecommerce` 一键 Chart。
详见 [architecture/cloud-native/README.md](architecture/cloud-native/README.md)。

## 环境说明

- Go ≥ 1.26
- Docker Desktop（跑 MySQL）
- Go 代理：`go env -w GOPROXY=https://goproxy.cn,direct`（国内网络必备）
