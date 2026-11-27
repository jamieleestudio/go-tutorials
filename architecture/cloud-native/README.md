# ⑥ 云原生架构 (Cloud-Native)

> 本期是**平台化升级**：治理中间件全部下沉到 K8s 平台，应用回归普通服务（无注册中心客户端、无网关依赖），遵循 12-Factor。
> 对应 java-tutorials 的 [`architecture/cloud-native`](../../../java-tutorials/architecture/cloud-native)。

## 与 P6 distributed 的关系（关键设计）

**业务代码逐字节相同，只换运维外壳** —— 这正是 Java 版 ③→⑥ 的设计（ArchUnit 验证过
`OrderServiceImpl` 等核心类零修改）。Go 版同构：domain/application/interfaces/infrastructure
四层与 RPC 客户端全部沿用 P6 的实现，变化只在：

| 能力 | P6 distributed | P7 cloud-native（本期） | Java 版做法 |
|---|---|---|---|
| 端口 | 8081/8082/8083 各不同 | **统一 :8080**（K8s 容器惯例，`PORT` env 覆盖） | `server.port: 8080` |
| 服务发现 | localhost 固定端口 | **K8s Service DNS**：`http://payment-service:8080` | 同左（无 Nacos） |
| 配置 | env + 默认值 | **12-Factor**：全部 env 注入（ConfigMap/Secret） | `${ENV:default}` |
| 健康检查 | 无 | **`/healthz/liveness` + `/healthz/readiness`** 双探针 | actuator probes |
| 指标 | 无 | **`/metrics`** Prometheus 文本（零依赖手写） | `/actuator/metrics` |
| 构建信息 | 无 | **`/info`**（版本来自 env） | `/actuator/info` |
| 优雅停机 | 只有裸 Shutdown | **摘流 → 排水 → 收尾** 完整序列 + K8s preStop | （Java 版缺失，Go 版增强） |
| 部署 | 手动 go run | **Dockerfile + K8s manifests + Helm Chart** | Jib + k8s + helm |

新增设施集中在 `shared/cloudkit`（对应 Java 的 actuator + 声明式配置，Go 没有等价开箱组件，手写 ~250 行标准库实现）。

## 模块结构（4 Go module + deploy）

```
cloud-native/
├── shared/                  # 内核 + persistence + cloudkit（云原生运行时设施）
│   └── cloudkit/            #   Health（liveness/readiness 门）· Metrics（Prometheus 文本）
│                            #   Serve（SIGTERM 优雅停机序列）· EnvOr（12-Factor 配置）
├── product/                 # 商品服务：REST :8080 + RPC /rpc/products（提供方）
├── payment/                 # 支付服务：REST :8080 + RPC /rpc/payments（pay 按 orderId 幂等）
├── order/                   # 订单服务：REST :8080，RPC 客户端基址 = K8s Service DNS
└── deploy/
    ├── docker/              # 3 个多阶段 Dockerfile（distroless 非 root 运行）
    ├── k8s/                 # Deployment+Service×3（探针/preStop/资源）+ configmap + secret
    │                        #   + ingress + hpa（2→6 副本 @ CPU 70%）
    └── helm/ecommerce/      # Chart：values + templates（deployment/service/ingress/secret）
```

## 探针语义（K8s 官方准则，Spring Actuator 同款）

| 端点 | 决策权 | 失败后果 | 语义要点 |
|---|---|---|---|
| `GET /healthz/liveness` | kubelet 重启容器 | pod 重启 | 进程能应答即 UP，**不检查数据库** —— 依赖挂了重启也没用 |
| `GET /healthz/readiness` | Service 摘流 | 不接新流量，不重启 | 启动中/停机中 → `503 {"status":"OUT_OF_SERVICE"}`（Spring 措辞） |

关键实现细节：就绪门是 `atomic.Bool`（探针 goroutine 与信号 goroutine 并发读写）；
bind 先于 ready 翻转（端口开着但 503 的窗口里 K8s 不放流量）。

## 优雅停机序列（Go 版增强，Java 版缺失项）

```
SIGTERM ──→ readiness=false（摘流，越早声明越少截断请求）
        ──→ server.Shutdown（8s 排水窗口，等存量请求完成）
        ──→ cleanup（关数据库 —— 此时已无业务请求）
        ──→ 进程退出（< terminationGracePeriodSeconds=30）
```

与 K8s 侧配成闭环：`preStop: sleep 5`（给 endpoints 摘除留时间）+ `terminationGracePeriodSeconds: 30`。
Java 版三份 K8s YAML 里没有这两项 —— deploy/ 注释里逐处标注了这是 Go 版补齐的。

## 本地运行（模拟 K8s：同一份代码、不同 env）

```powershell
make run-cn   # payment :8082 / product :8083 / order :8081（RPC 指向 localhost）

# 云原生端点（每个服务都有）
curl http://127.0.0.1:8082/healthz/liveness    # {"status":"UP"}
curl http://127.0.0.1:8082/healthz/readiness   # {"status":"UP"}
curl http://127.0.0.1:8082/metrics             # Prometheus 文本（含 http_requests_total）
curl http://127.0.0.1:8082/info

# 全链路（order → payment/product 是真实跨进程 HTTP RPC）
curl -X POST http://127.0.0.1:8083/api/products -H "Content-Type: application/json" `
  -d '{"name":"Laptop","price":"999.99","stock":10}'
curl -X POST http://127.0.0.1:8081/api/orders -H "Content-Type: application/json" `
  -d '{"customerId":"customer-1","lines":[{"productId":"<productId>","quantity":2}]}'
curl -X POST http://127.0.0.1:8081/api/orders/<orderId>/pay    # → PAID（远程支付）
```

在 K8s 里三服务都监听 8080，互访走 Service DNS —— 本地用 `PORT` 错开端口、
`PAYMENT_SERVICE_URL/PRODUCT_SERVICE_URL` 指回 localhost，**同一份镜像代码两种拓扑**，
这就是 12-Factor「配置外置」的演示。

## 构建与部署

```powershell
# 镜像（在 architecture/cloud-native 目录；distroless 非 root，二进制 ~15MB 无 JRE）
docker build -f deploy/docker/order-service.Dockerfile   -t go-tutorials/cn-order-service:1.0   .
docker build -f deploy/docker/payment-service.Dockerfile -t go-tutorials/cn-payment-service:1.0 .
docker build -f deploy/docker/product-service.Dockerfile -t go-tutorials/cn-product-service:1.0 .

# K8s（原生 manifests 或 Helm，二选一）
kubectl apply -f deploy/k8s/
helm install ecommerce deploy/helm/ecommerce
kubectl get pods,hpa,ingress
```

## 测试

```powershell
go test ./...                      # 逐模块跑（workspace 根跑 ./... 不生效）
go test -race ./shared/cloudkit    # 优雅停机序列：真 TCP 服务 + cancel ctx 验证摘流顺序
```

覆盖：领域单测（订单状态机/支付幂等）、订单流程（mock 远程服务，对应 @MockBean）、
端到端三服务栈（真 gin + 真 SQLite + 真 HTTP RPC：全链路支付/幂等重放/取消退款/RPC 契约）、
RPC 客户端规约（超时/只重试网络层错误）、架构守护（go list 扫 import，对应 ArchUnit 9 条规则）、
cloudkit 单测（探针门翻转、优雅停机序列顺序、指标格式、12-Factor 配置）。
本地无需 K8s / Docker —— 平台能力全部内化在部署层。
