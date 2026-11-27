package executor

import (
	"testing"
	"time"
)

func TestFixedPool(t *testing.T) {
	completed, used := FixedPoolDemo(5, 3, 10*time.Millisecond)
	if completed != 5 {
		t.Fatalf("completed=%d, want 5", completed)
	}
	if used < 1 || used > 3 {
		t.Fatalf("usedWorkers=%d, want <=3 distinct workers", used)
	}
}

func TestSingleWorkerOrder(t *testing.T) {
	order := SingleWorkerOrder(6)
	if len(order) != 6 {
		t.Fatalf("order=%v, want 6 entries", order)
	}
	for i, v := range order {
		if v != i+1 {
			t.Fatalf("single worker order broken: %v, want strict 1..6", order)
		}
	}
}

func TestCallerRuns(t *testing.T) {
	completed, callerRan := CallerRunsDemo(8, 2, 2, 30*time.Millisecond)
	if completed != 8 {
		t.Fatalf("completed=%d, want 8 (CallerRuns never drops)", completed)
	}
	if callerRan < 1 {
		t.Fatalf("callerRan=%d, want >=1 (8 slow jobs must overflow a 2+2 pool)", callerRan)
	}
}

func TestAfterFuncOnce(t *testing.T) {
	fired, elapsed := AfterFuncOnce(30*time.Millisecond, 10*time.Millisecond)
	if !fired {
		t.Fatal("AfterFunc did not fire")
	}
	if elapsed < 30*time.Millisecond {
		t.Fatalf("elapsed=%v, want >= delay 30ms", elapsed)
	}
}

func TestTickerSkips(t *testing.T) {
	runs, ideal := TickerSkipDemo(10*time.Millisecond, 30*time.Millisecond, 250*time.Millisecond)
	if ideal < 20 {
		t.Fatalf("ideal=%d (wall/period), test setup broken", ideal)
	}
	// 慢任务 30ms vs 周期 10ms：理想 25 tick，实际 ≤ wall/work ≈ 8。
	// 丢 tick 是 Go Ticker 的核心教学点（Java AtFixedRate 是"推迟不丢"）。
	if runs > ideal/3 {
		t.Fatalf("runs=%d ideal=%d: expected heavy tick skipping", runs, ideal)
	}
	if runs < 2 {
		t.Fatalf("runs=%d: too few, test timing broken", runs)
	}
}

func TestFixedDelay(t *testing.T) {
	gaps := FixedDelayDemo(20*time.Millisecond, 30*time.Millisecond, 3)
	if len(gaps) != 3 {
		t.Fatalf("gaps=%v, want 3", gaps)
	}
	for i, g := range gaps {
		if g < 45*time.Millisecond { // work 20 + gap 30 = 50ms，留 5ms 容差
			t.Fatalf("gap[%d]=%v, want >=50ms (work+delay)", i, g)
		}
	}
}

func TestGracefulDrain(t *testing.T) {
	if n := GracefulDrain(10, 2, 10*time.Millisecond); n != 10 {
		t.Fatalf("drain: completed=%d, want 10 (close does not drop queued jobs)", n)
	}
}

func TestForceShutdown(t *testing.T) {
	// 确定性编排：8 任务 / 2 worker，前 2 个快任务（30ms 完成），接着 2 个长任务
	// （卡死），其余 4 个排队。cancel 后状态被握手钉死，无 select 随机性。
	// 对应 Java shutdownNow 的三类收场：跑完的、被中断的、队列残留被丢弃的。
	completed, dropped := ForceShutdown(8, 2, 30*time.Millisecond)
	if completed != 2 {
		t.Fatalf("completed=%d, want exactly 2 (the quick jobs finish before cancel)", completed)
	}
	if dropped != 4 {
		t.Fatalf("dropped=%d, want exactly 4 (never picked after shutdownNow)", dropped)
	}
	interrupted := 8 - completed - dropped // 被取走又被中断的长任务
	if interrupted != 2 {
		t.Fatalf("interrupted=%d, want exactly 2 (the stuck long jobs)", interrupted)
	}
}

func TestSubmitAfterClose(t *testing.T) {
	if !SubmitAfterClose() {
		t.Fatal("submit to closed queue should be rejected")
	}
}
