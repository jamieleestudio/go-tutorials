# architecture/monolithic-multi-module —— Go 多模块单体（对应 Maven 多模块）

对 Java 版 `java-tutorials/architecture/monolithic-multi-module` 的 Go 重写。
**业务与 P1 完全一致**，唯一的架构变化是：把一个 Go module 拆成 **5 个 Go module**，
上下文边界从"包约定"升级为"模块边界"（编译期强制）。

## Maven module → Go module 对照

| Java (Maven) | Go (module) | 依赖 | 职责 |
|---|---|---|---|
| `mmm-shared-kernel` | `shared` | 无 | 内核：错误、聚合根基类、领域事件、EventBus、ID、事务传播原语 |
| `mmm-product` | `product` | shared | 产品上下文四层 |
| `mmm-payment` | `payment` | shared | 支付上下文四层 |
| `mmm-order` | `order` | shared + product + payment | 订单上下文四层 |
| `mmm-bootstrap` | `bootstrap` | 全部 | 组合根 + 全局错误中间件 + 入口 + 测试 |

依赖方向图（与 Java 版 pom 一致）：

```
            ┌──────────────┐
            │   shared     │ ← 纯内核，不依赖任何业务模块
            └──┬───────┬───┘
               │       │
        ┌──────▼─┐  ┌──▼──────┐
        │product │  │payment  │ ← 只依赖 shared
        └───┬────┘  └──┬──────┘
            │          │
            ┌──▼────────▼──┐
            │    order     │ ← 依赖三个模块，只用对方的 application 接口层
            └──────┬───────┘
                   │
            ┌──────▼───────┐
            │  bootstrap   │ ← 部署单元：组装 + 配置 + 错误映射 + 测试
            └──────────────┘
```

## 关键学习点

### 1. go.work = Maven reactor，go.mod require = pom `<dependency>`

- 根目录 `go.work` 用 `use` 聚合 6 个模块（P1 + P2 的 5 个）—— 对应 Maven 的父 POM `<module>` 聚合；
- 每个模块的 `go.mod` 里 `require github.com/.../monolithic-multi-module/order v0.0.0`
  是**编译期强制**的依赖声明 —— 没写 require 就 import，直接编译失败；
- `replace` 指令把模块指向本地兄弟目录 —— 对应 Maven reactor 的本地坐标解析
  （在 go.work 之外单独构建某模块时也需要它）。

**这是多模块版与 P1 的本质差异**：P1 的"order 不得依赖 product 的 infrastructure"是架构测试在守护；
P2 里"order 模块没 require product 模块就根本编译不过"，架构测试把约束细化到
"只允许 import 对方的 application 层"。

### 2. 循环依赖？Go 编译器免费送

Java 版 ArchUnit 有一条 `bounded_contexts_are_free_of_cycles` 规则。
Go 里**包级与模块级的 import 环都是编译错误** —— 想写一个循环依赖都做不到。
架构测试里只保留对依赖方向的断言，无需检测环。

### 3. 业务模块不能依赖 bootstrap

P1 里 controller 用了 `platform/httpmw.BindJSON`；多模块版里 platform 属于 bootstrap，
而 order 模块依赖 bootstrap 会造成环（bootstrap → order → bootstrap）。
对应 Java 版的做法：**校验逻辑留在模块内**（jakarta @Valid），
**错误→HTTP 映射在 bootstrap**（@RestControllerAdvice）。
Go 版把绑定校验内联进 controller（`c.ShouldBindJSON` + 抛 `VALIDATION_ERROR`）。

### 4. 数据库换了：MySQL → SQLite 内存库

Java 版 P2 特意从 MySQL 换成 H2 内存库（这个阶段重点是模块化，不是数据库）。
Go 版对应地默认使用 **SQLite 共享内存库**（纯 Go 驱动 `glebarez/sqlite`，无 CGO）：

```go
DB_DSN 默认 = "sqlite://file:mmm?mode=memory&cache=shared"   // 对应 jdbc:h2:mem:mmm;DB_CLOSE_DELAY=-1
```

`go run ./cmd/mmm` **零外部依赖**即可启动；要连真实 MySQL 时设 `DB_DSN` 覆盖
（可与 P1 共用同一个 docker-compose）。注意 SQLite 内存库必须限制单连接
（每个连接是独立库），已在 `bootstrap/persistence` 处理。

### 5. shared 模块承载了"Spring 框架能力"的 Go 对应物

Java 版 shared-kernel 零框架依赖，因为事务传播（@Transactional）和事件发布
（ApplicationEventPublisher）由 Spring 运行时提供。Go 没有容器，这两个原语
必须有个家 —— 放在 shared 模块的 `shared/persistence` 子包（TxRunner + context 传播）。
这是 Go 多模块的常见实践：**用基础模块承载框架适配层**（类比 Spring Boot starter）。
shared 根包保持纯净（仅 uuid），架构测试分别守护两层。

### 6. 组装从"扫描"变"手写"

Java 版 bootstrap 靠 `@ComponentScan` 反射发现所有模块的组件；
Go 版在 `bootstrap/app.go` 手动装配 —— 每一行都写明了"哪个模块的哪个实现被注入到哪里"。

## 目录结构

```
monolithic-multi-module/
├── shared/                  # Go module：内核
│   ├── errors.go / aggregate_root.go / domain_event.go / event_bus.go / id_generator.go / tx_runner.go
│   └── persistence/tx.go    # GormTxRunner + DBFromContext（事务 context 传播）
├── product/                 # Go module：产品上下文（domain/application/interfaces/infrastructure）
├── payment/                 # Go module：支付上下文（同上）
├── order/                   # Go module：订单上下文（同上 + domain/event）
└── bootstrap/               # Go module：部署单元
    ├── app.go               # 组合根（手动 DI）
    ├── config/              # 环境变量配置（对应 application.yml）
    ├── httpmw/              # 全局错误中间件（对应 GlobalExceptionHandler）
    ├── persistence/         # 数据库连接（SQLite/MySQL 路由）+ AutoMigrate
    ├── cmd/mmm/main.go      # 入口 + 优雅停机
    ├── integration_test.go  # 全流程集成测试（SQLite，零依赖）
    └── architecture_rules_test.go  # 架构守护（go.mod require 规则 + import 规则）
```

## 运行

```powershell
# 启动（零依赖：SQLite 内存库，无需 Docker）
go run ./architecture/monolithic-multi-module/bootstrap/cmd/mmm    # 端口 8081

# 测试（全部离线可跑）
go test ./...    # 在各模块目录下分别执行

# 切换真实 MySQL（可选，与 P1 共用 compose）
$env:DB_DSN = "monolith:monolith@tcp(localhost:13306)/mmm?charset=utf8mb4&parseTime=True&loc=Local"
```

环境变量：`APP_PORT`（默认 8081）、`DB_DSN`（sqlite:// 开头走 SQLite，其他按 MySQL DSN）、`GORM_LOG_LEVEL`。

## 架构守护规则（bootstrap/architecture_rules_test.go）

| # | 规则 | 层级 | Java 版对应 |
|---|---|---|---|
| 1 | shared 不 require 任何业务模块 | **go.mod（编译期）** | shared_kernel 纯净 |
| 2 | product/payment 只 require shared | **go.mod（编译期）** | pom 依赖声明 |
| 3 | order require shared+product+payment，且只 import 对方 application 层 | go.mod + import | order_context 规则 |
| 4 | bootstrap require 全部（部署单元） | **go.mod（编译期）** | pom 依赖声明 |
| 5 | domain 纯净（只允许 stdlib/decimal/uuid/shared） | import | domain 纯净性 |
| 6 | interfaces 不碰 infrastructure/impl | import | 分层方向 |
| 7 | infrastructure/impl 不碰 interfaces | import | 分层方向 |
| 8 | product/payment 不依赖 order、互相独立 | import | 上下文隔离 |
| 9 | shared 根包不依赖业务模块与 gin | import | shared_kernel_is_pure_java |
| — | 无循环依赖 | **Go 编译器保证** | bounded_contexts_are_free_of_cycles（免费获得） |

## P1 vs P2 测试放置差异

Java 版把所有测试放 bootstrap 模块（Maven 测试聚合）。Go 版遵循惯例：
**领域单测放各业务模块**（如 `order/domain/order_test.go`），**集成测试与架构守护放 bootstrap**
（它们需要装配全部模块）。语义完全一致，位置符合各语言生态惯例。
