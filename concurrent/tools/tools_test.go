package tools

import (
	"strings"
	"testing"
	"time"
)

func TestLatch(t *testing.T) {
	done := LatchDemo(3, 20*time.Millisecond)
	if len(done) != 3 {
		t.Fatalf("done=%v, want 3 workers", done)
	}
	// 全员完成后才返回（WaitGroup = CountDownLatch.await）
	if !strings.Contains(done[0], "finished") {
		t.Fatalf("entries: %v", done)
	}
}

func TestPhaser(t *testing.T) {
	timeline := PhaserDemo(3, 2)
	if len(timeline) != 6 {
		t.Fatalf("timeline=%v, want 6 events (3 parties × 2 phases)", timeline)
	}
	firstPhase1 := -1
	for i, e := range timeline {
		if strings.Contains(e, "phase-1") {
			firstPhase1 = i
			break
		}
	}
	if firstPhase1 < 0 {
		t.Fatalf("no phase-1 events: %v", timeline)
	}
	// 阶段栅栏：所有 phase-0 事件必须先于第一个 phase-1 事件
	for i := 0; i < firstPhase1; i++ {
		if !strings.Contains(timeline[i], "phase-0") {
			t.Fatalf("phase barrier broken: %v", timeline)
		}
	}
}

func TestBarrierRounds(t *testing.T) {
	actions, ok := BarrierRounds(3, 2)
	if !ok {
		t.Fatal("barrier rounds did not complete (reuse broken)")
	}
	if actions != 2 {
		t.Fatalf("barrierAction ran %d times, want 2 (once per round, by last arriver)", actions)
	}
}

func TestBarrierBroken(t *testing.T) {
	if n := BarrierBroken(3); n != 2 {
		t.Fatalf("broken barrier: %d waiters got error, want 2", n)
	}
}

func TestSemaphore(t *testing.T) {
	peak, completed := SemaphoreDemo(6, 2, 25*time.Millisecond)
	if completed != 6 {
		t.Fatalf("completed=%d, want 6", completed)
	}
	if peak != 2 {
		t.Fatalf("peak concurrency=%d, want exactly 2 (channel capacity IS the cap)", peak)
	}
}

func TestExchanger(t *testing.T) {
	t1Got, t2Got := ExchangerDemo()
	if t1Got != "B" || t2Got != "A" {
		t.Fatalf("exchanger: t1=%q t2=%q, want B/A", t1Got, t2Got)
	}
}

func TestSynchronousQueue(t *testing.T) {
	events := SynchronousQueueDemo()
	joined := strings.Join(events, " ")
	// 零缓冲的结构性保证：生产者发 Y 之前，消费者必须已拿走 X
	gotX := strings.Index(joined, "got-X")
	sentY := strings.Index(joined, "sent-Y")
	if gotX < 0 || sentY < 0 || gotX > sentY {
		t.Fatalf("zero-buffer guarantee broken: %v", events)
	}
	if !strings.Contains(joined, "got-Y") {
		t.Fatalf("missing got-Y: %v", events)
	}
}

func TestPoisonPill(t *testing.T) {
	consumed := PoisonPillPipeline(9)
	if len(consumed) != 9 {
		t.Fatalf("consumed=%v, want 9 items then pill", consumed)
	}
	for i, v := range consumed {
		if v != i+1 {
			t.Fatalf("consumed=%v, want 1..9 in order", consumed)
		}
	}
}

func TestRWMutex(t *testing.T) {
	peak, value := RWMutexDemo()
	if peak != 3 {
		t.Fatalf("peak concurrent readers=%d, want 3 (shared read lock)", peak)
	}
	if value != 42 {
		t.Fatalf("value=%d, want 42", value)
	}
}

func TestUniqueIDs(t *testing.T) {
	ids := UniqueIDs(4, 10)
	if len(ids) != 40 {
		t.Fatalf("len=%d, want 40", len(ids))
	}
	for i, id := range ids {
		if id != int64(i+1) {
			t.Fatalf("ids=%v: value %d duplicated/missing (atomic allocation broken)", ids, id)
		}
	}
}
