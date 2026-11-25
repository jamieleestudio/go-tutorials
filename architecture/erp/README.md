# architecture/erp —— ERP 可演进架构（平台层 + api 契约 + 端分包 + 多端装配）

对 Java 版 `java-tutorials/architecture/erp` 的 Go 重写。

Java 版把一个 42 模块的遗留 ERP 规范化为：**五层结构 + 端分包 + 域间只走 api + 包优先 + ArchUnit 护栏**
（35 个 Maven 工程 / 19 个业务域，其中 17 个是骨架域）。
Go 版遵循同样的"包优先"哲学：**整域一个 Go module**、**包结构逐层对照 Java**，
展开 3 个代表性域 —— `system`（纯提供方）+ `grade`（被跨域调用）+ `attendance`（完整样例，8 类端齐全）。
其余 16 个骨架域（student/enrollment/dormitory/hr/finance/workflow 等）不重复展开，
它们演示的架构概念已被这 3 个域完整覆盖。

## 五层结构（每域相同）

| 层 | 职责 | 可依赖 |
|---|---|---|
| `interfaces` | 端分包（web/admin/mobile/openapi/internalapi/device/mq/job/provider） | application |
| `application` | 用例编排（跨域调 api 契约 + 事务 + 事件） | domain、其它域 api、平台层 |
| `domain` | 聚合根/领域服务端口/领域事件/仓储端口 | 只允许 stdlib + shared |
| `infrastructure` | 持久化实现、外部系统适配器 | domain、平台层 |
| `api` | **对外契约**（接口 + DTO + 命令 + topic），零实现层依赖 | 只允许 stdlib |

依赖方向由架构护栏测试强制（见下文规则表）。

## 平台层（platform，6 个包，对应 Java 版 6 个子模块）

| 包 | 内容 | Java 对应 |
|---|---|---|
| `platform/kernel` | Result/PageResult/ErrorCode/BizException/**TenantContext（ctx value）** | erp-platform-kernel |
| `platform/persistence` | BaseModel embed / **TxRunner+DBFromContext（@Transactional 的 Go 对应物）** / SQLite·MySQL Open | erp-platform-persistence |
| `platform/security` | TokenService / ApiAuthFilter（解析 Bearer）/ AuthMiddleware（只拦 `/openapi/` 前缀） | erp-platform-security |
| `platform/web` | ErrorHandler（**404/422/400/500 → Result 包装**） | erp-platform-web |
| `platform/messaging` | MessageEnvelope / MessagePublisher / MessageConsumer / InMemory 分发 | erp-platform-messaging |
| `platform/integration` | ExternalSystemClient 端口 + HTTP 实现 | erp-platform-integration |

## 域间协作：只走 api 契约

```
attendance.application ──→ system.api（CurrentTenantID）     ← interfaces/provider 实现
        │                └→ grade.api（FindByStudentID）      ← interfaces/provider 实现
        └→ platform/messaging（publish erp.attendance.clocked）
```

签到用例（`attendance/application`）：打通租户 → 可选人脸校验（domain 端口 → infrastructure 适配器）
→ 跨域查成绩 → 领域工厂 `model.ClockIn`（PRESENT/LATE 规则 + 领域事件）→ 事务内持久化 → 发布事件信封。

## 端分包与多端装配（EndSet）

attendance 的 8 类端，各端 DTO 独立定义（端与端互不依赖）：

| 端 | 路径前缀 | 能力 |
|---|---|---|
| web | `/api/v1/attendance` | 签到 + 查询 |
| admin | `/api/v1/admin/attendance` | 只读 |
| mobile | `/api/v1/mobile/attendance` | 签到 + 查询 |
| openapi | `/openapi/v1/attendance` | 只读，受 `X-App-Id` 校验 |
| internalapi | `/internal/v1/attendance` | 只读（服务间） |
| device | `/openapi/v1/device/attendance/clock` | 考勤机上报（带 faceToken） |
| mq | ——（MessageConsumer） | 消费 `erp.attendance.clocked` |
| job | ——（goroutine + timer） | 每天 02:00 例行检查，仅 boot 启动 |

四种部署形态（对应 Java 版 4 个 app 模块）：

| 入口 | 端口 | 暴露的端 | Java 装配方式 |
|---|---|---|---|
| `app/cmd/erpboot` | 8080 | 全部 8 端 | `@EnableScheduling` |
| `app/cmd/erpadmin` | 8081 | web + admin | excludeFilters 排除其余 6 端 |
| `app/cmd/erpmobile` | 8082 | mobile | excludeFilters 排除其余 7 端 |
| `app/cmd/erpopenapi` | 8083 | openapi + device | excludeFilters 排除其余 6 端 |

Java 用 `@ComponentScan(excludeFilters = REGEX 正则)` 排除端；Go 没有类路径扫描，
**`assemble.EndSet` 里不写 true，就"不注册那行路由代码"** —— 效果相同，机制显式。

## 关键学习点（Go 特有的设计决策）

### 1. Go 结构化类型 vs 契约"只被 provider 实现"

Java 版规则：`XxxApi` 接口只能被 `interfaces.provider` 实现，不许应用服务自己实现。
**Go 是结构化类型** —— 只要方法签名一致，应用服务就会"无意中实现"契约，调用方可以绕过 provider。
两道防线：

- 应用层拥有自己的类型（`application.GradeView` / `application.ClockInCommand`），与
  `api.GradeDto` / `api.ClockInCommand` 分离 —— 签名天然不同，无意实现不可能发生；
  这也把 Java 版注释里"契约与本地用例相互独立"变成了真实的结构事实。
- 架构护栏测试用 **go/types 类型检查器**全模块扫描：任何实现了 `*Api` 接口的具名类型，
  必须位于 `interfaces/provider`（对应 ArchUnit 的 `getAllRawInterfaces` 反射扫描）。

### 2. `internal` 端撞上 Go 的 internal 可见性

Java 的 internal 端目录在 Go 里不能叫 `internal` —— Go 编译器把它当作强制可见性边界
（只有 attendance 域内代码可导入），组合根就无法注册路由了。Go 版改名 `interfaces/internalapi`。
这是两种语言可见性语义的真实碰撞点。

### 3. ThreadLocal → context.Context

Java 的 `TenantContext` 用 ThreadLocal 存"当前请求的租户"；Go 的 handler 可能跑在任意
goroutine 上，等价物是 **ctx value**（`kernel.WithTenant/CurrentTenant`），随请求 ctx 传递、天然不串。
同理 `@Transactional` → `TxRunner.WithinTx(ctx)`（事务句柄藏在 ctx 里传播），`@Scheduled` →
goroutine + timer（只在 boot 组合根启动，集成测试不启动）。

### 4. 错误映射：业务规则用 422

与 P1/P2 的 400 不同，erp 对齐 Java 版：`ENTITY_NOT_FOUND`→404、`BUSINESS_RULE_VIOLATION`→**422**、
校验失败→400、鉴权→401、其余→500；错误响应统一 `Result{success,code,message,data}` 包装，
成功响应直接返回端自己的 DTO（与 Java 版控制器一致）。

### 5. SQLite 内存库对应 H2

Java 版各 app 用 `jdbc:h2:mem:erp;DB_CLOSE_DELAY=-1`；Go 版默认
`sqlite://file:erp?mode=memory&cache=shared`（纯 Go 驱动，无 CGO，`go run` 零外部依赖）。
`DB_DSN` 可切 MySQL（注意 SQLite 内存库必须单连接，`platform/persistence.Open` 已处理）。

## 目录结构

```
erp/
├── shared/                          # 共享内核（零框架依赖，仅 uuid 豁免）
│   ├── errors.go                    #   DomainError：BUSINESS_RULE / ENTITY_NOT_FOUND
│   ├── aggregate_root.go            #   聚合根基类（RegisterEvent）
│   ├── domain_event.go / id_generator.go
├── platform/                        # 平台层 6 包（见上表）
├── system/                          # 纯提供方域：api → provider → application → domain → persistence
├── grade/                           # 被 attendance 跨域调用：同五层 + interfaces/web
├── attendance/                      # 完整样例：五层 + 8 类端 + 领域事件 + 人脸识别端口
│   ├── api/                         #   契约：AttendanceApi / ClockInCommand / TopicClocked
│   ├── domain/{model,event,service,repository}/
│   ├── application/                 #   签到用例（事务 + 跨域 + 事件）
│   ├── infrastructure/{persistence,client}/
│   └── interfaces/{web,admin,mobile,openapi,internalapi,device,mq,job,provider}/
└── app/
    ├── assemble/                    # 组合根：EndSet 多端装配 + 装配顺序即依赖顺序
    │   ├── assemble.go
    │   ├── integration_test.go      # SQLite 内存库 + httptest 全链路（4+2 用例）
    │   └── architecture_rules_test.go  # 架构护栏（import 分析 + go/types 类型检查器）
    └── cmd/{erpboot,erpadmin,erpmobile,erpopenapi}/main.go
```

## 运行

```powershell
# boot 形态：全部端（SQLite 内存库，零外部依赖）
go run ./architecture/erp/app/cmd/erpboot          # 端口 8080

# 其余形态
go run ./architecture/erp/app/cmd/erpadmin         # 8081 web+admin
go run ./architecture/erp/app/cmd/erpmobile        # 8082 mobile
go run ./architecture/erp/app/cmd/erpopenapi       # 8083 openapi+device

# 测试（全部离线可跑）
go test ./...    # 在 architecture/erp 下执行

# 演示链路
curl -X POST http://localhost:8080/api/v1/grades -H "Content-Type: application/json" ^
  -d '{"studentId":"stu-42","courseName":"math","score":88}'
curl -X POST http://localhost:8080/api/v1/attendance/clock-in -H "Content-Type: application/json" ^
  -d '{"studentId":"stu-42","clockInTime":"2026-09-16T08:00:00"}'
curl http://localhost:8080/openapi/v1/attendance?studentId=stu-42

# 切换真实 MySQL（可选）
$env:DB_DSN = "monolith:monolith@tcp(localhost:13306)/erp?charset=utf8mb4&parseTime=True&loc=Local"
# 开启 openapi 鉴权（缺 X-App-Id 返回 401）
$env:SECURITY_ENABLED = "true"
```

环境变量：`APP_PORT`、`DB_DSN`、`GORM_LOG_LEVEL`、`SECURITY_ENABLED`（默认 false）、`EXTERNAL_SYSTEM_URL`
（默认空 = 未配置外部系统，人脸校验走降级规则）。

## 架构护栏规则（app/assemble/architecture_rules_test.go）

| # | 规则 | 检查手段 | Java 对应 |
|---|---|---|---|
| 1 | 分层方向：interfaces 不被任何层访问；application 只被 interfaces 访问；domain 只被 application/infrastructure 访问；infrastructure 只被 application 访问 | go list import | LayerRulesTest.layered_dependencies |
| 2 | api 契约包不依赖任何实现层 | go list import | LayerRulesTest.api_packages... |
| 3 | `*Api` 契约接口只被 interfaces/provider 实现（防结构化类型"无意实现"） | **go/types 类型检查器** | ApiContractRulesTest |
| 4 | 跨域依赖只走对方 api 包 | go list import | DomainIsolationRulesTest |
| 5 | 无循环依赖 | **Go 编译器保证** | slices.beFreeOfCycles（免费获得） |
| 6 | domain 纯净：无 gin/gorm、无第三方库、不碰其他实现层 | go list import | DomainPurityRulesTest |
| 7 | 端隔离：8 类端互不依赖 | go list import | EndIsolationRulesTest |
| 8 | platform 不依赖业务域 | go list import | PlatformRulesTest |
| 9 | shared 内核纯净（仅 stdlib + uuid） | go list import | PlatformRulesTest.shared_kernel |
| 10 | system 是纯提供方（不依赖其他域） | go list import | PlatformRulesTest.system_domain |
