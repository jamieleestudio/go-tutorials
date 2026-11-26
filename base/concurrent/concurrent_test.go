package concurrent

import (
	"slices"
	"testing"
	"time"
)

func TestWaitGroupGoroutines(t *testing.T) {
	if got := WaitGroupGoroutines(100); got != 100 {
		t.Fatalf("got %d, want 100", got)
	}
}

func TestCounters(t *testing.T) {
	if got := MutexCounter(100, 1000); got != 100000 {
		t.Fatalf("mutex counter lost updates: got %d, want 100000", got)
	}
	if got := AtomicCounter(100, 1000); got != 100000 {
		t.Fatalf("atomic counter: got %d, want 100000", got)
	}
}

func TestChannelBasics(t *testing.T) {
	if got := Unbuffered(); got != "交接完成" {
		t.Fatalf("unbuffered: %q", got)
	}
	length, capacity, received := Buffered()
	if length != 2 || capacity != 3 || received != 3 {
		t.Fatalf("buffered: len=%d cap=%d received=%d", length, capacity, received)
	}
	sent, receivedOK, emptyProbe := SelectDefault()
	if !sent || !receivedOK || !emptyProbe {
		t.Fatalf("select default: %v %v %v", sent, receivedOK, emptyProbe)
	}
}

func TestRangeChannel(t *testing.T) {
	if got := RangeChannel(); got != 15 {
		t.Fatalf("range channel: got %d, want 15", got)
	}
}

func TestPipeline(t *testing.T) {
	if got := Pipeline(); got != 6 {
		t.Fatalf("pipeline: got %d, want 6", got)
	}
}

func TestSelectMultiplex(t *testing.T) {
	got := SelectMultiplex()
	slices.Sort(got) // 到达顺序不确定，排序后比对内容
	want := []string{"A1", "A2", "B1", "B2"}
	if !slices.Equal(got, want) {
		t.Fatalf("multiplex: %v, want %v", got, want)
	}
}

func TestTimeoutBySelect(t *testing.T) {
	if !TimeoutBySelect(1*time.Millisecond, 200*time.Millisecond) {
		t.Fatal("produce at 1ms should be received within 200ms")
	}
	if TimeoutBySelect(200*time.Millisecond, 5*time.Millisecond) {
		t.Fatal("produce at 200ms should time out within 5ms")
	}
}

func TestProduceConsume(t *testing.T) {
	if got := ProduceConsume(); got != 15 {
		t.Fatalf("directional channel: got %d, want 15", got)
	}
}

func TestWorkerPool(t *testing.T) {
	got := WorkerPool(4, 10)
	slices.Sort(got) // 完成顺序不确定
	want := []int{1, 4, 9, 16, 25, 36, 49, 64, 81, 100}
	if !slices.Equal(got, want) {
		t.Fatalf("worker pool: %v, want %v", got, want)
	}
}

func TestSyncPrimitives(t *testing.T) {
	if got := MutexDemo(10000); got != 10000 {
		t.Fatalf("mutex demo: %d, want 10000", got)
	}
	if got := CheckAndRemove(10000); got != 1 {
		t.Fatalf("check-and-remove: %d, want exactly 1", got)
	}
	held, acquired := TryLockDemo()
	if held || !acquired {
		t.Fatalf("trylock: held=%v acquired=%v", held, acquired)
	}
	value, sharedReads := RWMutexDemo()
	if value != 42 || sharedReads != 3 {
		t.Fatalf("rwmutex: value=%d sharedReads=%d", value, sharedReads)
	}
	if calls := OnceDemo(50); calls != 1 {
		t.Fatalf("once: %d calls, want 1", calls)
	}
}

func TestContextCancel(t *testing.T) {
	canceled, ticks := CancelDemo()
	if !canceled || ticks != 1 {
		t.Fatalf("cancel demo: canceled=%v ticks=%d, want true/1", canceled, ticks)
	}
}

func TestContextTimeout(t *testing.T) {
	if TimeoutDemo(200 * time.Millisecond) {
		t.Fatal("200ms limit should be enough for 50ms work")
	}
	if !TimeoutDemo(5 * time.Millisecond) {
		t.Fatal("5ms limit should time out for 50ms work")
	}
}

func TestContextValue(t *testing.T) {
	user, request := ValueDemo()
	if user != "小明" || request != "trace-2026" {
		t.Fatalf("value demo: user=%q request=%q", user, request)
	}
}
