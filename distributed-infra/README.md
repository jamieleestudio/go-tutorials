# distributed-infra：分布式基础设施（分布式锁 ×3 介质 + Outbox 完整版 + 雪花 ID）

参照 [java-tutorials/distributed-infra](../../../java-tutorials/distributed-infra) 的 Go 重写。
Java 版 23 个 Java 文件分布：distributed-lock 7 个 + distributed-solutions-outbox 13 main + 3 test；
**distributed-id / distributed-transaction 是只有 TODO 注释的空壳 pom**。Go 版把空壳里
排第一的 TODO（雪花算法）直接落地；transaction 不另起包（P5 `architecture/event-driven`
已覆盖 Saga + Outbox，见文末差异表）。

## 与 Java 版的模块映射

| Java 子模块 | Go 包 | 内容 |
|---|---|---|
| distributed-id（空壳 pom） | `distid/` | 雪花算法落地：41/10/12 bit 布局、时钟回拨保守拒绝、Decompose 反解 |
| distributed-lock/mysql | `distlock/mysql/` | 唯一索引锁（INSERT 抢占）+ 悲观锁（FOR UPDATE），裸 database/sql |
| distributed-lock/redis | `distlock/redis/` | SET NX PX 原子加锁 + Lua compare-and-del 原子解锁（go-redis） |
| distributed-lock/zookeeper | `distlock/zookeeper/` | 手写临时顺序节点 + watch 前驱（Java 用 Curator 黑盒，Go 版展开算法） |
| distributed-solutions-outbox | `outbox/` + `outbox/demo/` | Outbox 完整语义版：SKIP LOCKED 认领 + 指数退避 + FAILED 死信 |
| distributed-transaction（空壳 pom） | — | Seata/TCC 属 P8 后进阶；P5 的 Saga（编排式）已入仓库 |

## 教学主轴：同一把锁接口 × 三种介质 × 三种失效模式

`distlock.DistLock` 统一接口（`TryLock / Lock / Unlock`，对应 `java.util.concurrent.locks.Lock`
的教学子集），三种介质各自演示一种**失效模式**：

| 介质 | 加锁 | 释放 | 崩溃后果（失效模式） |
|---|---|---|---|
| MySQL | 唯一索引 INSERT / `FOR UPDATE` | DELETE 自己的行 / commit | **行残留 → 锁卡死**：无过期，需人工清理 |
| Redis | `SET NX PX` + TTL | Lua compare-and-del | **过期自愈**：但长任务超时丢锁（无看门狗续期） |
| ZooKeeper | 临时顺序节点 | 删除节点 | **会话自愈**：会话断开节点自清理，最优雅、性能最低 |

三个实现共享 `distlock.SpinTryLock / SpinTryLockForever` 自旋骨架 —— Java 版三个 demo
里一模一样的 `tryLock 失败 → parkNanos(100ms) → deadline → 重试` 粘贴三份，Go 版提炼成
一个 helper（`ctx` 取消即退出，对应 Java 的 isInterrupted 检查）。

共同纪律是**防误删**：Redis 解锁 Lua 脚本先比 value 再 DEL、MySQL 解锁 DELETE 带 owner_id
条件 —— 同一思想在两种介质上的投影（ZK 天然只有本人能删自己的节点）。

## 包结构

```
distributed-infra/
├── distid/                        # 雪花算法（纯标准库）
│   ├── snowflake.go               #   1+41+10+12 bit、epoch=2025-01-01、ErrClockMovedBackwards
│   └── snowflake_test.go          #   16 goroutine × 5000 并发唯一性、时钟回拨、nodeID 范围
├── distlock/
│   ├── lock.go                    #   DistLock 接口 + SpinTryLock/SpinTryLockForever + NewID
│   ├── mysql/                     #   唯一索引锁 + 悲观锁（真 MySQL 33061，不可达 skip）
│   ├── redis/                     #   RedisLock（miniredis 内嵌测试 = Java H2 零设施思路）
│   └── zookeeper/                 #   手写 ZK 锁算法（go-zookeeper，不可达 skip）
├── outbox/
│   ├── message.go                 #   Message 实体 + Status 状态机 + Pending 工厂
│   ├── schema.go                  #   Connect（建库建表）+ 两张表的仓库（对应 JPA Repository）
│   ├── publisher.go               #   Publisher SPI + LoggingPublisher + DemoHandler（故障注入）
│   ├── relay.go                   #   Relay：claimBatch SKIP LOCKED + relayOnce + 指数退避
│   ├── relay_test.go              #   5 个集成用例（Java OutboxRelayTest 1:1，真 MySQL）
│   ├── relay_unit_test.go         #   纯逻辑单测：退避公式 / 状态机 / 配置默认值（不连库）
│   └── demo/main.go               #   三场景演示（对应 OutboxDemoRunner）
└── docker-compose.yml             #   MySQL 33061 + ZooKeeper 2181 + Redis 6379（后两者可选）
```

## distid —— 雪花算法（Java 空壳 TODO 落地）

```
1 bit 符号（恒 0） | 41 bit 毫秒时间戳 | 10 bit 节点 ID | 12 bit 序列
```

- **时钟回拨**：NTP 校准回退 → 可能重复发号。本包保守策略直接返回 `ErrClockMovedBackwards`；
  生产可改「等待追平」或「备用位/扩展位」
- **序列耗尽**：单毫秒 4096 个用完 → 自旋等下一毫秒
- **节点 ID 分配**：手工分配易撞（测试里 `New(-1)`/`New(1024)` 直接拒绝）；生产可用
  DB/Redis/etcd 租约分配
- Java TODO 里另外三个方案的 Go 对照：UUID（`crypto/rand` 零依赖）、号段模式（atomic
  区间 + 预取）、Leaf（号段 + 雪花混合的工程封装）

## distlock —— 三种介质

### mysql（唯一索引锁 + 悲观锁）

- **UniqueIndexLock**：`INSERT` 成功即持锁、撞 1062 = 别人持有；解锁 `DELETE WHERE
  owner_id = 自己`。教学点：**无过期 → 崩溃残留、锁永久卡死**，需人工清理
- **PessimisticLock**：`SELECT ... FOR UPDATE` 行锁，生命周期完全绑定事务（Unlock =
  commit）。Java 版注释里点过的局限如实保留：没用 NOWAIT，后来者阻塞在 SELECT 上；
  真正的非阻塞升级是 MySQL 8.0+ 的 `FOR UPDATE NOWAIT/SKIP LOCKED`（outbox 用的正是后者）
- 测试连真 MySQL（33061），不可达自动 skip —— **Go 生态没有 H2 MODE=MySQL 的等价物**
  （SQLite 缺 FOR UPDATE/SKIP LOCKED），这是与 Java 版 demo 零设施的最大差异
- 连接模式坑：`database/sql` 连接池化，`USE` 只影响单个连接 —— `Connect` 建库后用
  `mysql.ParseDSN` 填库名重开连接池（不能手工字符串拼库名，DSN 尾部查询参数会坑掉它）

### redis（SET NX PX + Lua）

- 不是裸 SETNX：`SETNX + EXPIRE` 两步非原子，客户端崩溃后锁无 TTL 永久卡死；
  `SET NX PX` 一条命令原子完成「不存在才放 + 设过期」
- Lua compare-and-del：持有者阻塞超 lease → 锁过期易主 → 旧持有者醒来必须**显式感知
  解锁失败**（返回错误），否则就误删了别人的锁
- 看门狗续期是 Redisson 的增值特性，手写版不实现 —— 长任务超时丢锁是 Redis 锁固有风险
- 测试用 **miniredis 内嵌实例**（支持 SET NX PX 与 EVAL Lua）—— 这正是 Java 版
  MysqlLockDemo 用 H2「零外部设施跑通」的同款思路；`mr.FastForward` 确定性快进 TTL，
  过期自愈测试不用真等
- Java 版坑的规避：demo 在 10 线程间共享同一个 Jedis 实例（连接非线程安全）；
  go-redis 天然连接池化，无此坑

### zookeeper（手写临时顺序节点算法）

Java 版是 Curator `InterProcessMutex` 黑盒；Go 生态没有 Curator 级封装是常态，
本包用 go-zookeeper 原始客户端**手写算法**，把黑盒展开成四个教学点：

1. **临时顺序节点**：`/locks/{name}/lock-000000000N`，会话断开自动清理（持有者崩溃自愈）
2. **公平锁**：序列号最小者持锁，后来者排队（ZK 序列号零填充，字典序 == 数值序）
3. **只 watch 前驱**（羊群效应的解法）：若所有人 watch「锁节点删除」，锁一释放全体被
   唤醒重新竞争（羊群）；只监听自己的前驱，释放时只有一个 goroutine 被唤醒 —— O(1) 唤醒
4. **真非阻塞 TryLock**：非首位时**删除自己的排队节点**再返回 false（对应 Curator 的
   `acquire(0, MILLISECONDS)`），测试直接断言目录里只剩持有者的节点

Go 生态的惯用替代是 etcd（`clientv3 concurrency`：租约 + revision 排序 + watch 前驱，
与本算法完全同构）。

## outbox —— Outbox 完整语义版（本地消息表）

**解决的问题**：业务操作与其后续动作（执行本地业务 / 发 MQ）无法天然放进同一个事务——
先落业务再发消息，发送失败则丢事件；先发消息再落业务，消息先到而业务未落库。

```
业务事务（同一个本地事务，原子提交）:
    业务表写入（订单） + outbox 事件写入（PENDING）
              ↓
定时任务（relay.Run，ticker 轮询）:
    认领到期事件（FOR UPDATE SKIP LOCKED）
        → 逐条消费（Publisher.Publish）
              → 成功: status=PUBLISHED
              → 失败: retry_count+1, 指数退避 next_retry_at
                        → 超过 max_retries: status=FAILED（死信）
```

捞取到事件后**直接执行本地业务**（本模块 demo 的做法）与**投递到 MQ** 交给下游，
都是消费侧实现变体，模式骨架完全相同。

### 与 P5 `architecture/event-driven` 简化版的差异（为什么不互相替代）

| 维度 | P5 简化版 | 本包完整版 |
|---|---|---|
| 存储 | SQLite 内存库（零设施） | 真 MySQL 8（SKIP LOCKED 必须） |
| 认领 | FindPending 无认领语义，多实例会重复消费 | `FOR UPDATE SKIP LOCKED`，4 并发认领 20 条实证不重不漏 |
| 重试 | 无（投递失败下轮重投，无上限无退避） | 指数退避 `initial × multiplier^(retryCount-1)` + FAILED 死信 |
| 教学焦点 | Saga 编排 / 幂等去重 | SQL 语义本身（裸 database/sql，不引 GORM，每行 SQL 都看得见） |

### 核心机制（坑清单）

| 机制 | 实现与坑 |
|---|---|
| 原子性 | 业务数据与 outbox 记录同一本地事务写入，业务回滚则事件不存在（不多不丢） |
| 并发安全认领 | `SELECT ... WHERE status='PENDING' AND next_retry_at<=now ORDER BY id LIMIT ? FOR UPDATE SKIP LOCKED`（MySQL 8.0+）：多实例并发扫描互不阻塞、不重复认领 |
| 状态流转 | PENDING → PUBLISHED / FAILED；失败重试不占坑（退避期间不可被捞取） |
| 重试退避 | `next_retry_at = now + initial_backoff × multiplier^(retry_count-1)`，默认 5s 起、2 倍退避、5 次封顶；对照 Java `@ConfigurationProperties` 默认值，`Config.WithDefaults()` 同语义 |
| 至少一次 | 认领后进程宕机，记录仍是 PENDING，会被再次捞取 → **消费方必须幂等** |
| 索引 | `(status, next_retry_at)` 组合索引精确匹配扫描条件（新消息 `next_retry_at = created_at`，落库即到期） |
| 已知局限 | 认领与消费在同一事务，行锁持有时间受消费动作影响，batch-size 不宜过大；生产化方向是「认领 + PROCESSING 状态 + 超时回收」或 CDC（Debezium） |

SKIP LOCKED vs 乐观抢占：另一种认领写法是 `UPDATE ... LIMIT n` 靠影响行数抢占，少一次
查询但拿不到整行数据、且与 MySQL 的 LIMIT 语义有坑；SKIP LOCKED 语义清晰、行锁不阻塞
他人，是 MySQL 8.0+ 的推荐做法（本包采用）。

### 配置（`outbox.Config`）

| 字段 | 默认值 | 说明 |
|---|---|---|
| `PollInterval` | 1s | 轮询间隔（`relay.Run` 的 ticker） |
| `BatchSize` | 50 | 每批最多认领条数 |
| `MaxRetries` | 5 | 最大重试次数，达到后转 FAILED 死信 |
| `InitialBackoff` | 5s | 首次失败退避时间 |
| `BackoffMultiplier` | 2.0 | 退避倍数 |

### 测试用例（Java OutboxRelayTest 1:1，连真实 MySQL）

| 用例 | 验证点 |
|---|---|
| TestRelayOncePublishesDueMessageAndMarksPublished | 到期消息被消费并标记 PUBLISHED、publishedAt 落点 |
| TestRelayOnceSkipsMessagesNotDueYet | 未到期（next_retry_at 在未来）的消息不被捞取 |
| TestRelayOnceRetriesWithBackoffThenMarksFailed | 退避窗口内捞不到、retryCount 1→2→3、超限转 FAILED、投递恰 3 次 |
| TestConcurrentRelaysClaimDisjointBatches | 4 并发 relay 认领 20 条恰好各一次（BatchSize=10 < 20，SKIP LOCKED 生效） |
| TestRollbackLeavesNoOutboxRowAndNoBusinessRow | 同事务回滚后无事件、无业务数据泄漏 |

另有不连库的纯逻辑单测（relay_unit_test.go）：退避公式整条曲线、失败状态机、
配置默认值、Pending 工厂「落库即到期」。

### demo 三场景（`go run ./outbox/demo`）

1. **场景一**：订单与事件同事务提交 → relay 捞取 → 本地处理器执行 → PENDING→PUBLISHED
2. **场景二**：业务中途异常 → 同事务回滚 → outbox 无事件泄漏（对比回滚前后总行数）
3. **场景三**：处理器前两次失败 → 观察「第 N 次重试将于 Xs 后进行」指数退避日志（5s→10s）→ 第三次成功

## 运行

```powershell
# 1. 启动基础设施（MySQL 必须；ZK/Redis 供锁演示，测试探测不可达自动跳过）
docker compose -f distributed-infra/docker-compose.yml up -d

# 2. 跑测试（无基础设施时集成测试自动 skip，纯单测照常全绿）
go test ./distributed-infra/...
go test -race ./distributed-infra/...        # 全模块竞态验证

# 3. 跑 Outbox 三场景 demo（约 20s，观察指数退避日志）
go run ./distributed-infra/outbox/demo
```

连接串可用环境变量覆盖：`OUTBOX_MYSQL_DSN`（outbox）、`LOCK_MYSQL_DSN`（distlock/mysql）、
`LOCK_ZK_ADDR`（distlock/zookeeper）。DSN 形如
`root:rootpw@tcp(127.0.0.1:33061)/?parseTime=true&loc=UTC&timeout=3s`（**不含库名**，
`Connect` 会建库并返回绑定库名的连接）。

## Go 版对 Java 版的差异与增强

- **SpinTryLock 统一自旋骨架**：Java 三个锁 demo 的自旋循环粘贴三份，Go 提炼成一个 helper
- **ZK 黑盒展开**：Java 用 Curator 包装；Go 无等价库，手写算法反而把「临时顺序节点 +
  watch 前驱 + 羊群效应」变成了可读代码
- **miniredis 内嵌** = Java H2 内存库「零设施测试」的同款思路；MySQL 无内嵌替身 → 真
  MySQL + 探测 skip（Java assumeTrue 同款策略）
- **状态机拆纯函数**：`resolveSuccess/resolveFailure/NextBackoff` 不连库可单测；
  Java 这段逻辑藏在 handle 的 try/catch 里
- go-redis 客户端天然连接池化，Java「多线程共享 Jedis 实例」的坑在 Go 版不存在

## Roadmap（Java 版 TODO 原文 + outbox Roadmap）

- [ ] distributed-id 其余方案：号段模式 / Leaf / UUID 对照实现
- [ ] distributed-transaction：TCC / 消息事务 / 最大努力通知（P8 后进阶）
- [ ] distributed-solutions 其余方案：幂等消费表 / 数据对账 / 延迟任务
- [ ] KafkaOutboxPublisher：投递到 MQ 的消费实现
- [ ] FAILED 死信的重新入队管理接口；Redis 锁看门狗续期（Redisson 对照）
