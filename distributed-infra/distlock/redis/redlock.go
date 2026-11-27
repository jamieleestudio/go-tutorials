// Package redis 对应 java-tutorials/distributed-infra/distributed-lock/redis：
// JedisDistributedLock —— SET key value NX PX lease 原子加锁 + Lua 脚本原子解锁。
//
// 与 Java 版逐条对照（本包注释即 Java 类的教学点清单）：
//   - 不是裸 SETNX：裸 SETNX + EXPIRE 是两步非原子，客户端崩溃后锁无 TTL 永久卡死；
//     SET NX PX 一条命令原子完成「不存在才放 + 设过期」。
//   - value = 唯一标识（UUID）：解锁必须 compare-and-del —— 见 unlockScript。
//   - 有过期时间（lease）：持有者崩溃后锁自动过期（防死锁）；但长任务超时丢锁是
//     Redis 锁的固有风险 —— 续期（看门狗）是 Redisson 的增值特性，手写版不实现，
//     对照教学见测试与 README。
//   - Lua 解锁的原子性：GET+DEL 两步在并发下可能「检查完被易主、DEL 删了别人的锁」；
//     Lua 脚本在 Redis 单线程内执行，天然原子。
//   - Java 版坑的规避：demo 在 10 线程间共享同一个 Jedis 实例 —— Jedis 连接非线程安全；
//     go-redis 客户端天然连接池化，无此坑。
//
// 测试/演示用 miniredis（内嵌 Redis 实现，支持 SET NX PX 与 EVAL Lua）—— 这正是
// Java 版 MysqlLockDemo 用 H2 内存库「零外部设施跑通」的同款思路；生产连真 Redis
// （客户端代码不变，换 Addr 即可）。
package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/jamieleestudio/go-tutorials/distributed-infra/distlock"
)

// unlockScript Lua compare-and-del —— 本包的灵魂脚本（对应 Java UNLOCK_SCRIPT）：
// 只有 value 匹配（还是自己的锁）才 DEL，否则返回 0。
// 场景：持有者阻塞超过 lease → 锁过期 → 第二人拿锁 → 第一人醒来 DEL ——
// 没有 Lua 就会把别人的锁删掉；有 Lua 则这里返回 0，调用方显式感知「锁已易主」。
const unlockScript = `if redis.call('get', KEYS[1]) == ARGV[1] then
    return redis.call('del', KEYS[1])
else
    return 0
end`

// RedisLock 手写 Redis 分布式锁（对应 Java JedisDistributedLock）。
type RedisLock struct {
	rdb   redis.Cmdable // 接口类型：*redis.Client（真 Redis / miniredis）皆可
	key   string
	value string        // 持锁唯一标识（对应 Java 的 lockValue = UUID）
	lease time.Duration // 过期时间（对应 Java 的 leaseMillis，demo 用 10s）
}

// New 建锁。
func New(rdb redis.Cmdable, key string, lease time.Duration) *RedisLock {
	return &RedisLock{rdb: rdb, key: key, value: distlock.NewID(), lease: lease}
}

// TryLock 非阻塞抢占：SET key value NX PX lease（go-redis SetNX+TTL 的底层就是这条原子命令）。
// 返回 false = 锁被别人持有（对应 Java trySet 的 !"OK".equals(result)）。
func (l *RedisLock) TryLock(ctx context.Context) (bool, error) {
	ok, err := l.rdb.SetNX(ctx, l.key, l.value, l.lease).Result()
	if err != nil {
		return false, fmt.Errorf("set nx px %s: %w", l.key, err)
	}
	return ok, nil
}

// Lock 自旋重试直到拿到（对应 Java lock() 的 100ms parkNanos 骨架）。
func (l *RedisLock) Lock(ctx context.Context) error {
	return distlock.SpinTryLockForever(ctx, l.TryLock)
}

// Unlock Lua 原子解锁。锁已过期/易主时返回错误（对应 Java 抛 IllegalStateException
// —— 显式感知比静默成功安全：静默 = 误删别人的锁还以为释放成功）。
func (l *RedisLock) Unlock(ctx context.Context) error {
	n, err := l.rdb.Eval(ctx, unlockScript, []string{l.key}, l.value).Int64()
	if err != nil {
		return fmt.Errorf("eval unlock %s: %w", l.key, err)
	}
	if n == 0 {
		return fmt.Errorf("unlock failed, lock already released or expired: %s", l.key)
	}
	return nil
}
