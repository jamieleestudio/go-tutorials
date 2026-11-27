# concurrent —— Java j.u.c. 并发专题的 Go 对照

对应 [java-tutorials/concurrent](../../java-tutorials/concurrent) 的 7 个 maven 子模块
（callable / executor / future / sychronized / tools / virtual-thread / volatile），
Java 版零第三方依赖，Go 版同样**纯标准库**。

每个包 = 「带 Java↔Go 对照注释的源文件 + 测试断言」，测试即文档；
所有时序型坑（竞态窗口、死锁、丢 tick）用**通道编排确定性复现**，不用概率演示。

```powershell
go test ./concurrent/...               # 全部主题（普通构建：破损演示真跑，丢更新真实发生）
go test -race ./concurrent/...         # race 构建：破损演示由占位接管，修复版必须零竞争
```

## Java 7 子模块 ↔ Go 6 包

| Java 子模块 | Go 包 | 对照要点 |
|---|---|---|
| `sychronized` | `mutex` | synchronized 方法/代码块 ↔ `sync.Mutex`；DCL 四步演进 ↔ `sync.Once`；Go 特有：值复制陷阱（vet copylocks） |
| `volatile` | `volatile` | volatile 三句话 ↔ `sync/atomic`（seq-cst 即 acquire/release）；快照发布 ↔ `atomic.Pointer`；Go 特有防线：race detector |
| `executor` | `executor` | ThreadPoolExecutor 七参数 ↔ worker 数 + 有界 channel；四种拒绝策略 ↔ `Submit` 返回 false / `SubmitCallerRuns`；Scheduled ↔ `time.AfterFunc` / `time.Ticker`（丢 tick！） |
| `callable` + `future` | `future` | `Future.get` ↔ `<-ch`；`get(timeout)` ↔ `select + time.After`；`cancel(true)` ↔ `context.CancelFunc`；CompletionService ↔ 单结果 channel；CompletableFuture allOf/exceptionally ↔ 手写 errgroup 骨架 |
| `tools` | `tools` | CountDownLatch ↔ `WaitGroup`；CyclicBarrier ↔ 手写 `Barrier`（sync.Cond，含破损传播）；Semaphore ↔ 缓冲 channel；Exchanger/SynchronousQueue/BlockingQueue ↔ 三种 channel 形态；ReadWriteLock ↔ `sync.RWMutex` |
| `virtual-thread` | `virtthread` | Java 21 Loom ↔ Go 生而有之：per-task goroutine 性能实验；"不要池化虚拟线程" ↔ "给 goroutine 建池是反模式"；ScopedValue ↔ `context.WithValue` 的取舍 |

（callable 与 future 在 Java 里分两个模块讲；Go 版合并为一个包——goroutine + channel
本来就是一体的：`go f()` 是提交，`<-ch` 就是 `get()`，Java 的"任务/结果句柄"二分在 Go 不存在。）

## 学习顺序

1. **mutex** —— 锁语义第一课：锁必须绑定数据 → DCL 演进 → sync.Once 封装一切
2. **volatile** —— 可见性/有序性/原子性的边界 → atomic/channel/Once 的分工 → race detector
3. **executor** —— 任务提交与执行解耦：池参数、拒绝策略、Ticker 丢 tick、优雅关闭三步曲
4. **future** —— channel 即 Future：超时/取消/按完成序收集/池饥饿死锁
5. **tools** —— j.u.c. 工具箱逐个映射 channel 与 sync 原语
6. **virtthread** —— 压轴：Java 21 虚拟线程追上的，正是 Go 2009 年就有的一切

## 坑清单（逐条对照 Java 版笔记）

### mutex（← sychronized 两篇 md）
1. 实例锁错对象：每个任务 new 一个实例 → 每个实例一把锁 → 共享状态裸奔丢更新（`InstanceLockBroken`）
2. DCL 演进 1 裸单例：两线程同时通过判空 → 创建两次（`BareSingletonBroken`，通道编排钉死窗口）
3. DCL 演进 2 加锁不双检：第二个进锁不知道对象已建 → 仍重复创建（`LockedNoSecondCheckBroken`）
4. DCL 演进 3 的 volatile 之谜：`new` 三步（分配/构造/赋引用）2/3 可重排 → 需要 volatile 防 半成品发布
5. **Go 视角**：哪怕 Java 式 DCL 写对，锁外第一次判空在 Go 仍是数据竞争（-race 点名）→ 正解 `sync.Once`（`DCLCorrect` 注释 + `OnceSingleton`）
6. **Go 特有**：含锁结构体值复制 = 两把锁 —— vet copylocks 编译期拦截（`Counters` 注释，代码里不能真写）

### volatile（← volatile/README.md「3 句话」）
1. volatile 解决"看不见"和"乱序"，不解决"同时改" → 可见性用 atomic，互斥用 atomic.Add/Mutex
2. volatile 读 = acquire、写 = release → Go 的 atomic 默认 seq-cst；channel/锁也建立同样的 happens-before 边
3. "先读再写 / `++` / check-then-act / 复合条件"不是 volatile 能扛的 → `CheckThenActBroken`（编排复现）→ 临界区合并 / `LoadOrStore`
4. 丢更新静默发生：Java volatile++ 输出 lost > 0 才发现 → Go `go test -race` 当场报 DATA RACE（`PlainCounterBroken` 双构建的故事）
5. ReOrder 概率演示 → Go 版改为验证 happens-before 契约成立（channel 边 / Mutex 边），概率型输出不适合自动化测试
6. 该用的场景：停止标记（`StopFlagAtomic`，或惯用 `close(done)` 广播）、不可变快照发布（`atomic.Pointer[Config]`）、DCL 单例（→ sync.Once）

### executor（← EXECUTOR_LEARNING.md）
1. Executors 工厂 OOM 风险（Fixed 无界队列 / Cached 线程爆炸）→ Go 反而安全：有界 channel 满即感知；真正危险的是无上限 `go func`
2. `submit()` 异常不打印，必须 `future.get()` 或任务内捕获 → Go 对应：无人读的结果 channel = 无人 get 的 Future
3. 局部变量线程池 / 用完不关 → 线程泄露 → Go：`close(jobs)` 即 shutdown，`WaitGroup.Wait` 即 awaitTermination
4. 拒绝策略：Abort（返回 false）/ **CallerRuns（削峰填谷）**（`SubmitCallerRuns`）/ Discard —— 语义逐字对应
5. `scheduleAtFixedRate` 推迟不丢弃 vs Go `time.Ticker` **直接丢 tick** —— 慢任务请改"做完再 Reset"（`TickerSkipDemo` / `FixedDelayDemo`）
6. shutdownNow 三类收场：跑完的 / 被中断的 / 队列残留（`ForceShutdown` 确定性编排：2/2/4）
7. shutdown 后再 submit → Java RejectedExecutionException / Go **panic**（send on closed channel）→ 所有权设计：提交与关闭同锁互斥（`Submitter`）

### future（← callable/README.md 8 条 + FUTURE_LEARNING.md 4 条）
1. 无界阻塞：`get()` 不带超时 → `GetWithTimeout`（select + time.After）
2. 取消无效：`cancel(true)` 只是信号，任务不检查中断停不下来 → `CancelCooperative`：ctx.Done 必须被任务主动 select
3. 异常吞噬：ExecutionException 要 `getCause()` → Go error 是值，但无人读的 error 同样被吞（`ExceptionPropagation`）
4. 结果处理顺序：按提交序 get 效率低 → CompletionService ↔ 单结果 channel 先完成先出（`CompletionOrder`）
5. `invokeAll(timeout)` 返回的未完成任务已被取消 → `InvokeAll` 按提交序收集、超时标 Cancelled
6. **池饥饿死锁（Pool Induced Deadlock）**：池内任务阻塞等提交到同池的子任务 → `PoolStarvation`（2 worker 死锁 / 4 worker 通过）；Go 还有结构性出路：per-task goroutine 无池则无此坑
7. 滥用 get 退化成同步：提交完立刻取 = 串行 → `GetSerializationWrong/Right`（2× vs 1× 耗时）
8. 重计算浪费：结果未缓存 → 已完成态的 Future = 缓冲 1 的 channel 预填（`AllOfCached`）
9. 线程池泄漏（忘 shutdown）→ Go goroutine 结束即回收，无进程级泄漏，但 `Group.Wait` 仍是收口义务
10. CompletableFuture 链/allOf/exceptionally ↔ 同步函数 + 手写 `Group`（WaitGroup + sync.Once 记第一个错误；生产用 errgroup）

### tools（← tools/README.md 7 条）
1. 栅栏破损：一线程异常整轮 broken → 手写 `Barrier`（sync.Cond）：`Cancel` 让当前代全员拿到 `ErrBrokenBarrier`，可复用（generation +1）
2. 漏释放：acquire 未 release 长期阻塞 → `Semaphore` 缓冲 channel 实现，`defer Release`
3. 死锁或饥饿：互斥过多/资源争用 → 见 future 包的池饥饿死锁与 mutex 包的锁复制
4. 错误的中断处理：阻塞操作未响应中断 → ctx.Done 进 select（executor/future 包已演示）
5. 队列容量与背压：BlockingQueue 容量即背压 → 有界 channel 天然背压（`PoisonPillPipeline`）
6. 读写锁升级：持读锁再请求写锁 = 死锁 → `sync.RWMutex` 同样要先 RUnlock（`RWMutexDemo` 注释）
7. 参与者数量注册错误：Phaser await 永远不过 → Go 版"每阶段一个 WaitGroup"，数量错则全员阻塞（`PhaserDemo`）
8. 场景映射：等一组任务 → WaitGroup；阶段协作 → Barrier/Phaser 循环；限流 → Semaphore；严格配对交接 → 无缓冲 channel；毒丸终止 → 有界 channel + sentinel

### virtthread（← virtual-thread 类注释，无 md）
1. 虚拟线程按需创建、不池化，创建销毁开销极低 —— 这正是 goroutine 的定义；Go 版性能实验：1500 任务 × 40ms，per-task goroutine < 300ms vs 50 worker 池 ≈ 1.2s（Java 版 1 万任务 100s vs 1s 的缩影）
2. "不要池化虚拟线程" → Go 更进一步：给 goroutine 建池是反模式；池只在限流/背压时有意义（见 executor 包）
3. ScopedValue（JEP 446）三大优势 vs `context.WithValue`：不可变 ✓ / 无泄漏 △（没有作用域自动消失，靠 key 纪律）/ 深链开销 △（O(depth) 读，能显式传参就传参）
4. 载体线程卸载阻塞虚拟线程 ↔ Go GMP：goroutine 阻塞 syscall 时 P 交其他线程，殊途同归

## 目录

```
concurrent/
├── mutex/       # ← sychronized：锁语义、DCL 四步演进、值复制陷阱（broken*.go 按构建标签拆分）
├── volatile/    # ← volatile：可见性/原子性/happens-before/快照发布（notatomic_plain.go !race）
├── executor/    # ← executor：worker pool/拒绝策略/Ticker 丢 tick/优雅关闭三步曲
├── future/      # ← future+callable：channel 即 Future、池饥饿死锁、手写 errgroup
├── tools/       # ← tools：Latch/Barrier/Semaphore/Exchanger/毒丸/RWMutex/UniqueIDs
└── virtthread/  # ← virtual-thread：per-task goroutine 性能实验、ScopedValue ↔ context
```

## 关于构建标签的设计说明

「破损演示」的教学内容**本身就是数据竞争**——这正是 Go 相对 Java 的诊断优势：
Java 靠"输出 lost > 0"才发现问题（静默出错），Go 的 race detector 在写代码的当下
就点名竞争双方。因此这些演示按构建标签拆成两份：

- `//go:build !race`：真跑，丢更新/重复创建真实发生（`mutex/broken.go`、`volatile/notatomic_plain.go`）
- `//go:build race`：占位实现返回教学叙事期望的形状，让 `-race` 套件验证全部**修复版**零竞争

所以两种命令都要跑：不带 `-race` 看坑真实发生，带 `-race` 证明修复版干净。
