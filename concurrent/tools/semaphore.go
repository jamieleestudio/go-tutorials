package tools

import (
	"sync"
	"sync/atomic"
	"time"
)

// Semaphore 缓冲 channel 版信号量 —— 对应 Java SemaphoreExample。
//
// Java 版场景：2 permits / 4 线程 / 6 任务，最多同时 2 个 acquired。
// Go 的惯用实现就是"预填缓冲 channel"：Acquire 取一个空位，Release 归还。
// 生产项目也可以用 golang.org/x/sync/semaphore（带 context 的加权版本），
// 原理相同 —— 这个 10 行结构正是它的核心。
type Semaphore chan struct{}

// NewSemaphore 建 permits 个许可的信号量。
func NewSemaphore(permits int) Semaphore {
	s := make(Semaphore, permits)
	for range permits {
		s <- struct{}{} // 预填：channel 里的空位 = 可用许可
	}
	return s
}

// Acquire 取一个许可（无许可时阻塞，对应 Java acquire()）。
func (s Semaphore) Acquire() { <-s }

// TryAcquire 非阻塞尝试（对应 Java tryAcquire()）。
func (s Semaphore) TryAcquire() bool {
	select {
	case <-s:
		return true
	default:
		return false
	}
}

// Release 归还许可（对应 Java release()；Java README 坑 2"acquire 未 release 长期阻塞"
// 在 Go 版的防御：调用方务必 defer Release —— 测试里演示）。
func (s Semaphore) Release() { s <- struct{}{} }

// SemaphoreDemo 对应 Java 场景：permits 个许可跑 jobs 个任务，统计峰值并发。
// 峰值恰好等于 permits（结构上不可能超过 —— channel 容量就是上限）。
func SemaphoreDemo(jobs, permits int, work time.Duration) (peak int32, completed int) {
	s := NewSemaphore(permits)
	var (
		current atomic.Int32
		peakVal atomic.Int32
		wg      sync.WaitGroup
	)
	for range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Acquire()
			defer s.Release() // 对应 Java finally release
			c := current.Add(1)
			for { // CAS 抬高峰值
				old := peakVal.Load()
				if c <= old || peakVal.CompareAndSwap(old, c) {
					break
				}
			}
			time.Sleep(work)
			current.Add(-1)
		}()
	}
	wg.Wait()
	return peakVal.Load(), jobs
}
