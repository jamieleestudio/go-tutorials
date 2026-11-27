package future

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestLifecycle(t *testing.T) {
	events := Lifecycle()
	joined := strings.Join(events, " ")
	for _, want := range []string{"submitted", "doing-other", "probe-not-done", "task-done", "got:Hello, Future!"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("lifecycle events missing %q: %v", want, events)
		}
	}
	// 叙事顺序：提交 → 做别的 → 探测未完成 → 任务完成 → 取到结果
	if idx := strings.Index(joined, "submitted"); idx > strings.Index(joined, "got:") {
		t.Fatalf("narrative order broken: %v", events)
	}
}

func TestGetWithTimeout(t *testing.T) {
	ch := Submit(func() (string, error) {
		time.Sleep(50 * time.Millisecond)
		return "slow", nil
	})
	if _, ok := GetWithTimeout(ch, 5*time.Millisecond); ok {
		t.Fatal("5ms limit should time out for 50ms task")
	}
	if r, ok := GetWithTimeout(ch, time.Second); !ok || r.Value != "slow" {
		t.Fatalf("result: %v ok=%v", r, ok)
	}
}

func TestCancelCooperative(t *testing.T) {
	uninterruptible, cooperative := CancelCooperative()
	if !uninterruptible {
		t.Fatal("uninterruptible task should run to completion despite cancel (Java pitfall 2)")
	}
	if !cooperative {
		t.Fatal("context-aware task should stop early on cancel")
	}
}

func TestInvokeAll(t *testing.T) {
	out := InvokeAll(func(i int) (string, error) {
		time.Sleep(time.Duration(10+i*20) * time.Millisecond) // 10/30/50ms
		return "done", nil
	}, 3, 40*time.Millisecond) // 50ms 的任务赶不上期限
	if out[0].Cancelled || out[0].Value != "done" {
		t.Fatalf("fast task: %+v", out[0])
	}
	if !out[2].Cancelled {
		t.Fatalf("slow task should be cancelled at deadline: %+v", out[2])
	}
}

func TestExceptionPropagation(t *testing.T) {
	err := ExceptionPropagation()
	if err == nil || err.Error() != "boom" {
		t.Fatalf("err=%v, want boom (Java getCause 的 Go 对应物)", err)
	}
}

func TestCompletionOrder(t *testing.T) {
	// 耗时递减：id 0 最慢（50ms）、id 4 最快（10ms）→ 完成序应为 4,3,2,1,0
	order := CompletionOrder(func(i int) time.Duration {
		return time.Duration(50-i*10) * time.Millisecond
	}, 5)
	want := []int{4, 3, 2, 1, 0}
	for i, v := range want {
		if order[i] != v {
			t.Fatalf("completion order=%v, want %v (first-done-first-out)", order, want)
		}
	}
}

func TestChainAndCached(t *testing.T) {
	if got := ChainDemo(); got != "Order_123 -> Amount: $100 -> Invoice: Created" {
		t.Fatalf("chain: %q", got)
	}
	done, value := AllOfCached()
	if !done || value != "Cached Value (Immediate)" {
		t.Fatalf("cached future: done=%v value=%q", done, value)
	}
}

func TestParallelQueries(t *testing.T) {
	success, failed, err := ParallelQueries(context.Background())
	if success != 2 || failed != 1 {
		t.Fatalf("success=%d failed=%d, want 2/1", success, failed)
	}
	if err == nil || !strings.Contains(err.Error(), "db error") {
		t.Fatalf("firstErr=%v, want db error", err)
	}
}

func TestGetSerialization(t *testing.T) {
	const work = 40 * time.Millisecond
	wrong := GetSerializationWrong(work, 2)
	right := GetSerializationRight(work, 2)
	// 错法 ≈ 80ms（串行），对法 ≈ 40ms（并行）。断言留 20ms 容差防调度抖动。
	if wrong < work+work-20*time.Millisecond {
		t.Fatalf("wrong way should serialize: %v", wrong)
	}
	if right > work+20*time.Millisecond {
		t.Fatalf("right way should parallelize: %v", right)
	}
}

func TestPoolStarvation(t *testing.T) {
	// 池 = 2：3 个外层任务互相等 → 期限内必然死锁（教学核心坑 4）。
	if !PoolStarvation(600*time.Millisecond, 2) {
		t.Fatal("2-worker pool with nested same-pool waits should deadlock")
	}
	// 池 = 4：外层占 3 个，剩 1 个 worker 跑子任务 → 能全部完成。
	if PoolStarvation(600*time.Millisecond, 4) {
		t.Fatal("4-worker pool should survive the same workload")
	}
}

func TestFetchAll(t *testing.T) {
	results := FetchAll(context.Background(),
		[]string{"/ok", "/missing", "/slow"},
		50*time.Millisecond) // /slow 睡 300ms → 必超时
	byURL := map[string]WebFetchShape{}
	for _, r := range results {
		byURL[r.URL] = r
	}
	if r := byURL["/ok"]; r.Status != 200 || r.Bytes != len("hello concurrent") {
		t.Fatalf("/ok: %+v", r)
	}
	if r := byURL["/missing"]; r.Status != 404 {
		t.Fatalf("/missing: %+v", r)
	}
	if r := byURL["/slow"]; !r.Cancelled || r.Err == nil {
		t.Fatalf("/slow should be cancelled at timeout: %+v", r)
	}
}
