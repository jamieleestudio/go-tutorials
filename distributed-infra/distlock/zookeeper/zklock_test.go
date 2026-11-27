package zookeeper

import (
	"context"
	"net"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	zk "github.com/go-zookeeper/zk"
)

// testConn 探测 ZooKeeper 并返回连接。不可达自动跳过 —— 对应 Java 测试
// assumeTrue 的「探测跳过」策略（ZK 语义必须真服务验证，无内嵌替身可用）。
func testConn(t *testing.T) *zk.Conn {
	t.Helper()
	addr := envOr("LOCK_ZK_ADDR", "127.0.0.1:2181")

	// TCP 层先行探测：zk.Connect 对不可达主机会在后台持续重连，探测不过直接 skip
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Skipf("ZooKeeper 不可达，请先 docker compose -f distributed-infra/docker-compose.yml up -d：%v", err)
	}
	_ = conn.Close()

	zkc, err := Connect(addr, 5*time.Second)
	if err != nil {
		t.Skipf("ZooKeeper 会话建立失败：%v", err)
	}
	t.Cleanup(func() { zkc.Close() })
	return zkc
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func uniqueName(prefix string) string {
	return prefix + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
}

// TestCompete 8 goroutine 竞争同一把锁：临时顺序节点 + watch 前驱串行化 → counter == 8。
// （对应 Java ZookeeperLockDemo 的多线程竞争输出）
func TestCompete(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()

	var counter atomic.Int64
	var wg sync.WaitGroup
	const workers = 8
	name := uniqueName("resource_zk")
	guard := time.AfterFunc(30*time.Second, func() { panic("zk lock competition deadlocked after 30s") })
	defer guard.Stop()

	for i := range workers {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			lock := New(conn, name)
			if err := lock.Lock(ctx); err != nil {
				t.Errorf("worker %d Lock: %v", i, err)
				return
			}
			counter.Add(1)
			time.Sleep(20 * time.Millisecond) // 模拟临界区工作
			if err := lock.Unlock(ctx); err != nil {
				t.Errorf("worker %d Unlock: %v", i, err)
			}
		}()
	}
	wg.Wait()
	if counter.Load() != workers {
		t.Fatalf("counter=%d, want exactly %d（互斥失败 = 有并发进入临界区）", counter.Load(), workers)
	}
}

// TestTryLockWhileHeld TryLock 真非阻塞：被持有时立刻返回 false，且自己的排队节点已删除；
// 释放后再试成功。（对应 Java acquire(0, MILLISECONDS) 语义 + 临时节点清理纪律）
func TestTryLockWhileHeld(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()

	name := uniqueName("resource_try")
	holder := New(conn, name)
	if ok, err := holder.TryLock(ctx); err != nil || !ok {
		t.Fatalf("holder TryLock: ok=%v err=%v", ok, err)
	}

	competitor := New(conn, name)
	ok, err := competitor.TryLock(ctx)
	if err != nil {
		t.Fatalf("competitor TryLock: %v", err)
	}
	if ok {
		t.Fatal("TryLock must fail while held（真非阻塞，不排队等待）")
	}
	// 教学点：失败即撤队 —— 排队节点已删除，目录里只剩持有者的一个节点
	children, _, err := conn.Children(holder.dir)
	if err != nil {
		t.Fatalf("children: %v", err)
	}
	if len(children) != 1 {
		t.Fatalf("children=%v, want exactly 1 node（TryLock 失败必须删除自己的排队节点）", children)
	}

	// 释放后 → 立刻能拿到
	if err := holder.Unlock(ctx); err != nil {
		t.Fatalf("holder Unlock: %v", err)
	}
	if ok, err := competitor.TryLock(ctx); err != nil || !ok {
		t.Fatalf("TryLock after release: ok=%v err=%v", ok, err)
	}
	if err := competitor.Unlock(ctx); err != nil {
		t.Fatalf("competitor Unlock: %v", err)
	}
}

// TestLockFairness Lock 公平性：先到者持锁期间，第二个 goroutine 排队（watch 前驱）；
// 前驱释放后被唤醒拿到锁 —— 「只 watch 前驱」算法的行为验证。
func TestLockFairness(t *testing.T) {
	conn := testConn(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	name := uniqueName("resource_fair")
	first := New(conn, name)
	if err := first.Lock(ctx); err != nil {
		t.Fatalf("first Lock: %v", err)
	}

	var secondGot atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		second := New(conn, name)
		if err := second.Lock(ctx); err != nil {
			t.Errorf("second Lock: %v", err)
			return
		}
		secondGot.Store(true)
		_ = second.Unlock(ctx)
	}()

	time.Sleep(300 * time.Millisecond) // 让 second 完成建节点 + watch 前驱
	// 排队中：second 尚未持锁，目录里应有 2 个节点（持有者 + 排队者）
	children, _, err := conn.Children(first.dir)
	if err != nil {
		t.Fatalf("children: %v", err)
	}
	if len(children) != 2 {
		t.Fatalf("children=%d, want 2（second 应处于 watch 前驱的公平排队状态）", len(children))
	}
	if secondGot.Load() {
		t.Fatal("second must not acquire while first holds the lock")
	}
	if err := first.Unlock(ctx); err != nil {
		t.Fatalf("first Unlock: %v", err)
	}
	wg.Wait()
	if !secondGot.Load() {
		t.Fatal("second should acquire after predecessor released（watch 唤醒失效）")
	}
}
