package tools

import (
	"sort"
	"sync"
	"sync/atomic"
)

// 数据一致性工具 —— 对应 Java ReadWriteLockExample 与 AtomicIntegerExample。

// RWMutexDemo 对应 ReadWriteLockExample：1 写 + 3 读。
// 读锁共享（峰值并发读者 = 3），写锁独占。返回峰值并发读者与最终值。
func RWMutexDemo() (peakReaders int32, value int) {
	var (
		mu      sync.RWMutex
		data    int
		current atomic.Int32
		peak    atomic.Int32
		wg      sync.WaitGroup
	)

	// 1 写（对应 writeLock().lock/put/unlock）
	wg.Add(1)
	go func() {
		defer wg.Done()
		mu.Lock()
		data = 42
		mu.Unlock()
	}()
	wg.Wait()

	// 3 读并发：读锁共享 → 峰值同时读者应为 3
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.RLock()
			defer mu.RUnlock()
			c := current.Add(1)
			for { // CAS 抬高峰值
				old := peak.Load()
				if c <= old || peak.CompareAndSwap(old, c) {
					break
				}
			}
			_ = data // 读操作
		}()
	}
	wg.Wait()

	// Java README 坑 6 的注释位：读写锁升级（持读锁再请求写锁）在 Go 同样死锁 ——
	// RWMutex 的 RLock 与 Lock 互斥，持 RLock 时调 Lock 必卡死；正解：先 RUnlock 再 Lock。
	return peak.Load(), data
}

// UniqueIDs 对应 AtomicIntegerExample：4 线程 × 10 次 incrementAndGet。
// 返回全部分到的编号 —— 每个值恰好出现一次（原子自增的分配语义）。
func UniqueIDs(goroutines, iters int) []int64 {
	var (
		counter atomic.Int64
		slot    atomic.Int64 // 结果槽位分配器：每个下标只被写一次，无需加锁
	)
	n := goroutines * iters
	results := make([]int64, n)
	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range iters {
				id := counter.Add(1)        // incrementAndGet 的原子对应物
				results[slot.Add(1)-1] = id // 独占槽位写入
			}
		}()
	}
	wg.Wait()
	sort.Slice(results, func(i, j int) bool { return results[i] < results[j] })
	return results
}
