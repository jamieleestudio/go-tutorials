package volatile

import (
	"sync"
	"sync/atomic"
)

// AtomicCounter 修复 1（对应 Java AtomicIntegerExample）：atomic.Add 自带原子性。
// 注意语义差异：Java 的 volatile++ 丢更新，所以要用 AtomicInteger；Go 的
// atomic.AddInt64 本来就是原子的 —— "++ 不原子"这个坑在 Go 的 atomic 里不存在，
// 但"check-then-act 不原子"依然存在（见 CheckThenActBroken）。
func AtomicCounter(goroutines, iters int) int64 {
	var n atomic.Int64
	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range iters {
				n.Add(1)
			}
		}()
	}
	wg.Wait()
	return n.Load()
}

// MutexCounter 修复 2：Mutex 圈临界区（与 mutex 包呼应；写多读少时比 atomic 直白）。
func MutexCounter(goroutines, iters int) int {
	var mu sync.Mutex
	n := 0
	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range iters {
				mu.Lock()
				n++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return n
}

// CheckThenActBroken check-then-act 坑 —— Java README 第 3 句话的对应物：
// "先读再写 / ++ / check-then-act / 复合条件就不是 volatile 能单独扛住的"。
//
// 编排式确定性复现：两个 goroutine 都先完成"检查不存在"，再各自插入 → 重复创建。
// Java 对应场景：if (map.get(k) == null) map.put(k, v)；原子读 + 原子写拼起来不原子。
func CheckThenActBroken() int {
	var mu sync.Mutex
	seen := map[string]bool{}
	duplicates := make(chan struct{}, 4)
	checked := make(chan struct{}) // 报告"检查完成：不存在"
	release := make(chan struct{}) // 两人都检查完再写
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			exists := seen["k"] // 原子【读】（锁内）
			mu.Unlock()
			if !exists {
				checked <- struct{}{} // 窗口钉死：两人都检查过"不存在"
				<-release             // 等放行再写
				mu.Lock()             // 原子【写】（另一次加锁）
				seen["k"] = true      // 检查与动作之间隔着窗口 → 双方都插入
				mu.Unlock()
				duplicates <- struct{}{}
			}
		}()
	}
	<-checked
	<-checked
	release <- struct{}{}
	release <- struct{}{}
	wg.Wait()
	close(duplicates)
	return len(duplicates) // 2：同一 key 被插入两次
}

// CheckThenActFixed 修复：检查与动作放进同一个临界区（锁内复合判断）。
// 对应 Java 的 synchronized { if(!map.containsKey) put } 或 ConcurrentHashMap.putIfAbsent；
// Go 里若只想"存在即跳过"，还有一个无锁答案：sync.Map 的 LoadOrStore（内部 CAS）。
func CheckThenActFixed() int {
	var mu sync.Mutex
	seen := map[string]bool{}
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			if !seen["k"] { // 检查+动作原子化
				seen["k"] = true
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	return len(seen) // 1
}
