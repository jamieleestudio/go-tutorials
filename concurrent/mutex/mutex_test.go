package mutex

import "testing"

// 本包测试全部确定（编排式复现 + WaitGroup），无 sleep 时序依赖；
// 整个 concurrent 模块建议额外跑 go test -race（见 README）。

func TestInstanceLock(t *testing.T) {
	got, want := InstanceLockBroken(16, 1000)
	if got >= want {
		t.Fatalf("broken per-instance lock should lose updates: got=%d want=%d", got, want)
	}
	if n := InstanceLockCorrect(16, 1000); n != 16000 {
		t.Fatalf("correct shared lock: n=%d, want 16000", n)
	}
	if n := GuardedBlock(16, 1000); n != 16000 {
		t.Fatalf("guarded block: n=%d, want 16000", n)
	}
}

func TestCountersPointerReceiver(t *testing.T) {
	// 值复制陷阱的"防线上"演示：指针接收者才是同一把锁、同一份数据。
	// （陷阱本身的代码会被 go vet copylocks 直接拒绝 —— 见 mutex.go 的注释块，
	// 所以这里不能真的写 `snapshot := *c`：vet 会报 assignment copies lock value。
	// 并发场景下两把锁各改各的副本，原对象的 n 永远停在 0。）
	c := &Counters{}
	c.Inc(1000)
	if c.Value() != 1000 {
		t.Fatalf("pointer receiver: n=%d, want 1000", c.Value())
	}
}

func TestDCLEvolution(t *testing.T) {
	if n := BareSingletonBroken(); n != 2 {
		t.Fatalf("bare singleton: %d creations, want exactly 2 (both passed first check)", n)
	}
	if n := LockedNoSecondCheckBroken(); n != 2 {
		t.Fatalf("locked without second check: %d creations, want exactly 2", n)
	}
	n, value := DCLCorrect()
	if n != 1 || value != 42 {
		t.Fatalf("DCL correct: %d creations value=%d, want 1/42", n, value)
	}
}

func TestOnceSingleton(t *testing.T) {
	n, same := OnceSingleton(64)
	if n != 1 {
		t.Fatalf("sync.Once: %d constructions, want 1", n)
	}
	if !same {
		t.Fatal("sync.Once: callers saw different instances")
	}
}
