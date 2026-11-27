package redis

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/jamieleestudio/go-tutorials/distributed-infra/distlock"
)

// newTestRedis 起 miniredis 内嵌实例（Java 版 H2 MODE=MySQL 的同款零设施思路）。
func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}

// TestCompete 10 goroutine 竞争同一把锁：互斥串行化 → counter == 10。
// （对应 Java RedisLockDemo jedisLockDemo 的 10 线程竞争输出）
func TestCompete(t *testing.T) {
	mr, rdb := newTestRedis(t)
	ctx := context.Background()

	var counter atomic.Int64
	var wg sync.WaitGroup
	for i := range 10 {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			lock := New(rdb, "lock:jedis:resource", 10*time.Second)
			if err := lock.Lock(ctx); err != nil {
				t.Errorf("worker %d Lock: %v", i, err)
				return
			}
			counter.Add(1)
			time.Sleep(10 * time.Millisecond) // 模拟临界区工作
			if err := lock.Unlock(ctx); err != nil {
				t.Errorf("worker %d Unlock: %v", i, err)
			}
		}()
	}
	wg.Wait()
	if counter.Load() != 10 {
		t.Fatalf("counter=%d, want exactly 10（互斥失败 = 有并发进入临界区）", counter.Load())
	}
	_ = mr
}

// TestUnlockByNonOwner Lua compare-and-del 防误删：
// value 不匹配 → DEL 不执行 → 解锁报错（对应 Java unlock 抛 IllegalStateException）。
func TestUnlockByNonOwner(t *testing.T) {
	_, rdb := newTestRedis(t)
	ctx := context.Background()

	holder := New(rdb, "lock:owner", time.Minute)
	if ok, err := holder.TryLock(ctx); err != nil || !ok {
		t.Fatalf("holder TryLock: ok=%v err=%v", ok, err)
	}

	other := New(rdb, "lock:owner", time.Minute) // 同 key、不同 value
	if ok, _ := other.TryLock(ctx); ok {
		t.Fatal("SET NX on held key must fail")
	}
	if err := other.Unlock(ctx); err == nil {
		t.Fatal("unlock with mismatched value must error（否则 = 误删他人锁）")
	}
	// 持有者的锁完好无损：其他人仍然抢不到
	if ok, _ := other.TryLock(ctx); ok {
		t.Fatal("lock must still be held after failed foreign unlock")
	}
	if err := holder.Unlock(ctx); err != nil {
		t.Fatalf("holder unlock: %v", err)
	}
}

// TestLeaseExpiry TTL 过期自愈 —— Redis 锁的失效模式（对照 MySQL 残留 / ZK 会话）：
// 持有者「崩溃」不释放 → lease 到期锁消失 → 新持有者拿锁；
// 旧持有者醒来解锁 → 报错（锁已易主）—— 这就是为什么解锁必须检查返回。
func TestLeaseExpiry(t *testing.T) {
	mr, rdb := newTestRedis(t)
	ctx := context.Background()

	first := New(rdb, "lock:ttl", 100*time.Millisecond)
	if ok, err := first.TryLock(ctx); err != nil || !ok {
		t.Fatalf("first TryLock: ok=%v err=%v", ok, err)
	}

	// 时间快进 200ms：lease 过期（miniredis 的确定性时钟，不用真等）
	mr.FastForward(200 * time.Millisecond)

	second := New(rdb, "lock:ttl", time.Minute)
	ok, err := second.TryLock(ctx)
	if err != nil || !ok {
		t.Fatalf("second TryLock after expiry: ok=%v err=%v（过期自愈失败）", ok, err)
	}
	// 旧持有者醒来解锁：锁已易主 → 必须报错而不是静默成功
	if err := first.Unlock(ctx); err == nil {
		t.Fatal("expired holder unlock must error（compare-and-del 防误删失效）")
	}
	if err := second.Unlock(ctx); err != nil {
		t.Fatalf("second unlock: %v", err)
	}
}

// TestTryLockSpin TryLock 非阻塞 + SpinTryLock 到期退出（distlock.SpinTryLock 骨架）。
func TestTryLockSpin(t *testing.T) {
	_, rdb := newTestRedis(t)
	ctx := context.Background()

	holder := New(rdb, "lock:spin", time.Minute)
	if ok, err := holder.TryLock(ctx); err != nil || !ok {
		t.Fatalf("holder TryLock: ok=%v err=%v", ok, err)
	}
	defer func() { _ = holder.Unlock(ctx) }()

	competitor := New(rdb, "lock:spin", time.Minute)
	ok, err := distlock.SpinTryLock(ctx, 250*time.Millisecond, competitor.TryLock)
	if err != nil {
		t.Fatalf("SpinTryLock: %v", err)
	}
	if ok {
		t.Fatal("spin tryLock should time out while held")
	}
}
