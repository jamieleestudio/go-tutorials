package virtthread

import (
	"context"
	"testing"
	"time"
)

func TestCreate(t *testing.T) {
	if n := Create(64); n != 64 {
		t.Fatalf("created=%d, want 64 goroutines completed", n)
	}
}

func TestPerformance(t *testing.T) {
	const (
		tasks   = 1500
		work    = 40 * time.Millisecond
		workers = 50
	)
	perTask := PerTaskGoroutines(tasks, work)
	pooled := PoolThrottled(tasks, workers, work)
	// per-task ≈ work + 调度余量；池 ≈ tasks/workers × work = 30 × 40ms = 1.2s。
	// 断言留 4 倍余量，但方向必须成立（Java 版同样方向：100s vs 1s）。
	if perTask > 300*time.Millisecond {
		t.Fatalf("per-task goroutines: %v, want < 300ms (massive concurrency is free)", perTask)
	}
	if pooled < 4*work {
		t.Fatalf("pooled: %v, want >= 4×work (throttling must serialize work)", pooled)
	}
	t.Logf("per-task=%v pooled(50)=%v —— Java 1万任务 100s vs 1s 的 Go 版缩影", perTask, pooled)
}

func TestScoped(t *testing.T) {
	ctx := RequestScoped(context.Background(), "Alice")
	out := HandleRequest(ctx, "GET /api/user")
	// 对应 Java 输出：接收到请求 GET /api/user | 当前用户: Alice | 审计操作人: Alice
	for _, want := range []string{"GET /api/user", "Alice"} {
		if !contains(out, want) {
			t.Fatalf("scoped value missing %q: %s", want, out)
		}
	}
	if countOccurrences(out, "Alice") != 2 {
		t.Fatalf("deep chain should see user twice: %s", out)
	}
	if !UnboundOutsideScope() {
		t.Fatal("outside scope should be unbound (isBound=false 的 Go 形态)")
	}
}

// contains/countOccurrences 小工具（教学演示不引入 strings 了 —— 其实引入也无妨，
// 这里保持纯 context 叙事）。
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func countOccurrences(s, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			n++
			i += len(sub) - 1
		}
	}
	return n
}
