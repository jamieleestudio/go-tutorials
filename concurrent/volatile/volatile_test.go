package volatile

import (
	"testing"
	"time"
)

func TestStopFlag(t *testing.T) {
	spins, stoppedInTime := StopFlagAtomic(0, 20*time.Millisecond)
	if !stoppedInTime {
		t.Fatal("atomic.Bool flag: worker did not observe stop within 2s")
	}
	if spins == 0 {
		t.Fatal("atomic.Bool flag: worker never spun (test invalid)")
	}
	if !StopFlagChannel(0, 20*time.Millisecond) {
		t.Fatal("close(done) broadcast: worker did not stop within 2s")
	}
	// 裸 bool 版本的教训在注释里：它本身就是 DATA RACE，go test -race 会当场点名，
	// 而 Java 版普通 boolean 的失败形态是"worker 永远空转"（静默）。
}

func TestCounters(t *testing.T) {
	got, want := PlainCounterBroken(8, 20000)
	if got > want {
		t.Fatalf("plain counter impossible: got=%d want=%d", got, want)
	}
	if got != want {
		t.Log("race 构建外运行：丢更新确实发生 got=", got, "want=", want)
	}
	if n := AtomicCounter(8, 20000); n != 160000 {
		t.Fatalf("atomic counter: %d, want 160000", n)
	}
	if n := MutexCounter(8, 20000); n != 160000 {
		t.Fatalf("mutex counter: %d, want 160000", n)
	}
}

func TestCheckThenAct(t *testing.T) {
	if n := CheckThenActBroken(); n != 2 {
		t.Fatalf("check-then-act broken: %d inserts, want exactly 2 (both passed the check)", n)
	}
	if n := CheckThenActFixed(); n != 1 {
		t.Fatalf("check-then-act fixed: %d entries, want 1", n)
	}
}

func TestHappensBeforeContract(t *testing.T) {
	viaChannel, viaMutex := HappensBeforeContract()
	if viaChannel != 42 || viaMutex != 42 {
		t.Fatalf("happens-before edges: channel=%d mutex=%d, want 42/42", viaChannel, viaMutex)
	}
}

func TestSnapshotPublish(t *testing.T) {
	observed, torn, final := SnapshotPublish(80 * time.Millisecond)
	if torn {
		t.Fatal("atomic.Pointer publication: torn pair observed (release/acquire broken?)")
	}
	if observed == 0 {
		t.Fatal("snapshot publish: readers observed nothing (test invalid)")
	}
	if final.Name != "v2" || final.Version != 2 {
		t.Fatalf("final config: %s:%d, want v2:2", final.Name, final.Version)
	}
}

func TestOnceValueDCL(t *testing.T) {
	n, same := OnceValueDCL(32)
	if n != 1 {
		t.Fatalf("once-DCL: %d constructions, want 1", n)
	}
	if !same {
		t.Fatal("once-DCL: callers saw different values")
	}
}

// TestSnapshotValuesFused 快照值域断言：读到的每一份快照都在合法集合内。
func TestSnapshotValuesFused(t *testing.T) {
	valid := []*Config{{Name: "v1", Version: 1}, {Name: "v2", Version: 2}}
	for _, c := range valid {
		if !c.validPair() {
			t.Fatalf("valid pair rejected: %+v", c)
		}
	}
	torn := []Config{{Name: "v1", Version: 2}, {Name: "v2", Version: 1}}
	for _, c := range torn {
		cp := c
		if cp.validPair() {
			t.Fatalf("torn pair accepted: %+v", c)
		}
	}
}
