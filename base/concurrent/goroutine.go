// Package concurrent 并发编程：goroutine、channel、sync 原语与 context。
//
// 对应 java-tutorials 的 base/se/concurrent（ReentrantLockTest）。
// 主线迁移：Java 的 Thread/线程池/ReentrantLock/ConcurrentHashMap，
// 在 Go 里对应 goroutine + WaitGroup / channel / sync.Mutex / 带锁的 map。
package concurrent

import (
	"sync"
	"sync/atomic"
)

// WaitGroupGoroutines WaitGroup 等一组 goroutine 全部结束，atomic 汇总计数。
//
// 对照 Java：ExecutorService + CountDownLatch；Go 不搞"线程池"套路 ——
// goroutine 极轻（KB 级栈、创建成本极低），要多少开多少，同步交给 WaitGroup 或 channel。
func WaitGroupGoroutines(n int) int64 {
	var (
		wg    sync.WaitGroup
		total atomic.Int64
	)
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			total.Add(1) // 原子加：多 goroutine 并发安全（对应 Java 的 AtomicLong）
		}()
	}
	wg.Wait() // 阻塞直到计数器归零
	return total.Load()
}

// MutexCounter goroutines 个 goroutine 各自自增 times 次。
//
// 教学点：n++ 是"读-改-写"三步，不加锁会互相覆盖丢更新（数据竞争，
// `go test -race` 能直接抓出来）；加 Mutex 后每次自增都在临界区内，结果精确。
// 对照 Java：ReentrantLock.lock()/unlock()，但没有 finally —— Go 用 defer 保证解锁。
func MutexCounter(goroutines, times int) int {
	var (
		mu sync.Mutex
		n  int
		wg sync.WaitGroup
	)
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range times {
				mu.Lock()
				n++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return n
}

// AtomicCounter atomic.Int64：无锁原子自增，比 Mutex 更轻 ——
// 简单计数器优先用它（对应 Java 的 AtomicLong.incrementAndGet）。
func AtomicCounter(goroutines, times int) int64 {
	var (
		total atomic.Int64
		wg    sync.WaitGroup
	)
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range times {
				total.Add(1)
			}
		}()
	}
	wg.Wait()
	return total.Load()
}
