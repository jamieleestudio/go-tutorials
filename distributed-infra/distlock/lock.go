// Package distlock 对应 java-tutorials/distributed-infra/distributed-lock 子模块：
// 「同一 java.util.concurrent.locks.Lock 接口 × 三种介质」的对照实验。
//
// 三种介质 = 三种失效模式（本模块的教学主轴）：
//
//	介质        加锁                        释放                    崩溃后果（失效模式）
//	MySQL      唯一索引 INSERT / FOR UPDATE  DELETE 自己的行 / commit  行残留 → 锁卡死，需人工清理（无过期）
//	Redis      SET NX PX + TTL              Lua compare-and-del      过期自愈（但长任务可能丢锁；无看门狗）
//	ZooKeeper  临时顺序节点                  删除节点                 会话断开节点自清理（最优雅自愈）
//
// 递进关系：锁解决「谁先做」，outbox 包（distributed-solutions）解决「做了就绝不丢」。
//
// 子包映射：
//
//	mysql/      ← MysqlUniqueIndexLock + MysqlPessimisticLock（裸 database/sql，SQL 语义看得见）
//	redis/      ← JedisDistributedLock（SET NX PX + Lua；go-redis 客户端 + miniredis 内嵌演示）
//	zookeeper/  ← ZookeeperDistributedLock（Java 用 Curator 黑盒；Go 版手写临时顺序节点算法展开）
package distlock

import (
	"context"
	"crypto/rand"
	"time"
)

// DistLock 分布式锁统一接口（对应 java.util.concurrent.locks.Lock 的教学子集：
// tryLock(timeout) / unlock；lockCondition 不支持 —— 对应 Java 版 newCondition 一律抛异常）。
type DistLock interface {
	// TryLock 非阻塞尝试：拿到返回 true，没拿到（别人持有）返回 false。
	TryLock(ctx context.Context) (bool, error)
	// Lock 自旋重试直到拿到锁；ctx 取消/超时返回错误。
	Lock(ctx context.Context) error
	// Unlock 释放锁。实现必须防误删（只删自己持有的锁）。
	Unlock(ctx context.Context) error
}

// spinInterval 三个 Java 实现共享的自旋节奏（LockSupport.parkNanos(100ms) 的对应物）。
const spinInterval = 100 * time.Millisecond

// SpinTryLock 自旋骨架 —— 三个 Java demo 的 tryLock(time) 完全同构：
// tryOnce 失败 → 睡 100ms → deadline 检查 → 再试；ctx 取消即刻退出。
// 提炼成统一 helper 正是 Go 版对 Java 版「三处复制粘贴」的小改进。
func SpinTryLock(ctx context.Context, wait time.Duration, tryOnce func(ctx context.Context) (bool, error)) (bool, error) {
	deadline := time.Now().Add(wait)
	for {
		ok, err := tryOnce(ctx)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
		select {
		case <-ctx.Done(): // 中断检查（对应 Java 的 Thread.currentThread().isInterrupted()）
			return false, ctx.Err()
		case <-time.After(spinInterval): // 对应 LockSupport.parkNanos(100ms)
		}
		if time.Now().After(deadline) {
			return false, nil // tryLock 超时：没拿到
		}
	}
}

// SpinTryLockForever 无限期自旋版（对应 Java 的 lock()）：拿到锁或 ctx 取消才返回。
func SpinTryLockForever(ctx context.Context, tryOnce func(ctx context.Context) (bool, error)) error {
	for {
		ok, err := tryOnce(ctx)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(spinInterval):
		}
	}
}

// NewID 生成请求/持锁唯一标识（Java 版 UUID.randomUUID() 的零依赖对应物）。
// value/ownerID 的意义：解锁时带上它防误删 —— Redis 的 Lua compare-and-del 与
// MySQL 的 DELETE WHERE owner_id 是同一思想在两种介质上的投影。
func NewID() string {
	var b [16]byte
	_, _ = rand.Read(b[:]) // crypto/rand 不会失败；失败时也是随机内容，教学演示可接受
	const hex = "0123456789abcdef"
	out := make([]byte, 0, 36)
	for i, v := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			out = append(out, '-')
		}
		out = append(out, hex[v>>4], hex[v&0x0f])
	}
	return string(out)
}
