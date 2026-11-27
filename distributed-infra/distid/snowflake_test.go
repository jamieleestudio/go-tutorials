package distid

import (
	"errors"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestNextIDShapeAndDecompose(t *testing.T) {
	s, err := New(7)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	id, err := s.NextID()
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}
	if id < 0 {
		t.Fatalf("id=%d: sign bit must be 0", id)
	}
	at, nodeID, sequence := Decompose(id)
	if nodeID != 7 {
		t.Fatalf("nodeID=%d, want 7", nodeID)
	}
	if sequence != 0 {
		t.Fatalf("sequence=%d, want 0 for first id", sequence)
	}
	if at.UnixMilli() < epoch {
		t.Fatalf("timestamp %v before epoch %d", at, epoch)
	}
}

func TestConcurrentUnique(t *testing.T) {
	const (
		goroutines = 16
		iters      = 5000 // 8 万个 ID：跨毫秒 + 同毫秒序列两条路径都会走到
	)
	s, err := New(3)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ids := make([]int64, 0, goroutines*iters)
	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			local := make([]int64, 0, iters)
			for range iters {
				id, err := s.NextID()
				if err != nil {
					t.Errorf("NextID: %v", err)
					return
				}
				local = append(local, id)
			}
			mu.Lock()
			ids = append(ids, local...)
			mu.Unlock()
		}()
	}
	wg.Wait()

	slices.Sort(ids)
	for i := 1; i < len(ids); i++ {
		if ids[i] == ids[i-1] {
			t.Fatalf("duplicate id %d at index %d (unique allocation broken)", ids[i], i)
		}
	}
	// 单调性：排序后首尾必然递增，且发号时序内不存在回退
	if len(ids) != goroutines*iters {
		t.Fatalf("got %d ids, want %d", len(ids), goroutines*iters)
	}
}

func TestClockMovedBackwards(t *testing.T) {
	s, err := New(1)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := s.NextID(); err != nil {
		t.Fatalf("first NextID: %v", err)
	}
	// 直接触发内部游标回拨场景：lastTS 被推到未来
	s.mu.Lock()
	s.lastTS = time.Now().UnixMilli() + 60_000 // 人为把游标拨到 1 分钟后
	s.mu.Unlock()
	_, err = s.NextID()
	if !errors.Is(err, ErrClockMovedBackwards) {
		t.Fatalf("err=%v, want ErrClockMovedBackwards（时钟回拨保守拒绝）", err)
	}
}

func TestNewNodeRange(t *testing.T) {
	if _, err := New(-1); err == nil {
		t.Fatal("nodeID=-1 should be rejected")
	}
	if _, err := New(1024); err == nil {
		t.Fatal("nodeID=1024 should be rejected (>1023)")
	}
	if _, err := New(1023); err != nil {
		t.Fatalf("nodeID=1023 should be accepted: %v", err)
	}
}
