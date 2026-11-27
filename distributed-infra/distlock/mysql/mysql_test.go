package mysql

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jamieleestudio/go-tutorials/distributed-infra/distlock"
)

// competitor 竞争者需要的最小接口（两种锁都实现）。
type competitor interface {
	Lock(ctx context.Context) error
	Unlock(ctx context.Context) error
}

// testDB 探测 MySQL 并返回绑定 distributed_infra 的连接。
// 不可达自动跳过 —— 与 Java 版 Redis/ZK demo 的「探测跳过」同款策略
// （Go 生态没有 H2 MODE=MySQL 的等价物，MySQL 语义必须真库验证）。
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	serverDSN := envOr("LOCK_MYSQL_DSN", "root:rootpw@tcp(127.0.0.1:33061)/?parseTime=true&loc=UTC&timeout=3s")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := Connect(ctx, serverDSN)
	if err != nil {
		t.Skipf("MySQL 不可达，请先 docker compose -f distributed-infra/docker-compose.yml up -d：%v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// runCompetitors 10 goroutine 竞争同一把锁：拿到 → 计数 → 短暂持有 → 释放。
// 全部串行化则 counter == 10（对应 Java demo 的 counter 输出）。
func runCompetitors(t *testing.T, newLock func() competitor, hold time.Duration) int64 {
	t.Helper()
	var counter atomic.Int64
	var wg sync.WaitGroup
	ctx := context.Background()

	// 死锁守卫：MySQL 锁等待超时默认 50s，竞争若卡死 30s 即炸出 panic（测试二进制直接失败）
	guard := time.AfterFunc(30*time.Second, func() { panic("lock competition deadlocked after 30s") })
	defer guard.Stop()

	for i := range 10 {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			lock := newLock()
			if err := lock.Lock(ctx); err != nil {
				t.Errorf("worker %d Lock: %v", i, err)
				return
			}
			counter.Add(1)
			time.Sleep(hold)
			if err := lock.Unlock(ctx); err != nil {
				t.Errorf("worker %d Unlock: %v", i, err)
			}
		}()
	}
	wg.Wait()
	return counter.Load()
}

func TestUniqueIndexLock(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	lockName := "resource_ui_" + time.Now().Format("150405.000000000")
	counter := runCompetitors(t, func() competitor {
		return NewUniqueIndexLock(db, lockName)
	}, 10*time.Millisecond)
	if counter != 10 {
		t.Fatalf("counter=%d, want exactly 10（互斥失败 = 有并发进入临界区）", counter)
	}

	// 教学点：防误删 —— 解锁 DELETE 带 owner_id 条件，非 owner 删不掉
	owner := NewUniqueIndexLock(db, lockName+"_owner")
	if ok, err := owner.TryLock(ctx); err != nil || !ok {
		t.Fatalf("fresh lock should succeed: ok=%v err=%v", ok, err)
	}
	other := NewUniqueIndexLock(db, lockName+"_owner") // 不同 ownerID，同名锁
	if ok, _ := other.TryLock(ctx); ok {
		t.Fatal("duplicate name must fail: unique index conflict = lock held")
	}
	if err := other.Unlock(ctx); err == nil {
		t.Fatal("unlock by non-owner must fail（DELETE owner_id 条件 = 防误删）")
	}
	if err := owner.Unlock(ctx); err != nil {
		t.Fatalf("owner unlock: %v", err)
	}
}

func TestPessimisticLock(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	lockName := "resource_b_" + time.Now().Format("150405.000000000")
	if err := SeedRow(ctx, db, lockName); err != nil {
		t.Fatalf("seed row: %v", err)
	}
	counter := runCompetitors(t, func() competitor {
		return NewPessimisticLock(db, lockName)
	}, 10*time.Millisecond)
	if counter != 10 {
		t.Fatalf("counter=%d, want exactly 10（FOR UPDATE 行锁互斥失败）", counter)
	}

	// 教学点：行必须预先存在 —— FOR UPDATE 锁的是已存在的行（对应 Java 预插 resource_B）
	missing := NewPessimisticLock(db, lockName+"_missing")
	if ok, err := missing.TryLock(ctx); ok || err != nil {
		t.Fatalf("FOR UPDATE on missing row: ok=%v err=%v, want false/nil", ok, err)
	}
}

func TestTryLockTimeout(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	lockName := "resource_try_" + time.Now().Format("150405.000000000")
	holder := NewUniqueIndexLock(db, lockName)
	if ok, err := holder.TryLock(ctx); err != nil || !ok {
		t.Fatalf("holder TryLock: ok=%v err=%v", ok, err)
	}
	defer func() { _ = holder.Unlock(ctx) }()

	competitor := NewUniqueIndexLock(db, lockName)
	ok, err := distlock.SpinTryLock(ctx, 300*time.Millisecond, competitor.TryLock)
	if err != nil {
		t.Fatalf("SpinTryLock: %v", err)
	}
	if ok {
		t.Fatal("tryLock should time out while holder keeps the lock")
	}
	// 释放后再试 → 成功
	if err := holder.Unlock(ctx); err != nil {
		t.Fatalf("release: %v", err)
	}
	ok, err = distlock.SpinTryLock(ctx, time.Second, competitor.TryLock)
	if err != nil || !ok {
		t.Fatalf("tryLock after release: ok=%v err=%v", ok, err)
	}
}
