package future

import (
	"sync/atomic"
	"time"
)

// 坑演示 —— 对应 java-tutorials/future/FuturePitfallDemo 与 FUTURE_LEARNING.md 的四大坑。
//
//	坑 1 滥用 get 退化成同步        → GetSerialization（错法 2×耗时 vs 对法 1×耗时）
//	坑 2 cancel 停不下来            → future.go 的 CancelCooperative
//	坑 3 异常被吞                   → future.go 的 ExceptionPropagation
//	坑 4 线程池枯竭（Pool Induced Deadlock）→ PoolStarvation（本文件的教学核心）

// GetSerializationWrong 对应 FuturePitfallDemo 坑 1 的错误示范：
// 提交 f1 → 立刻 get → 再提交 f2 → get。第二个任务在第一个 get 期间根本没提交，
// 总耗时 = 各任务耗时之和（Java 版注释：Expected ~2000ms）。
func GetSerializationWrong(work time.Duration, tasks int) time.Duration {
	start := time.Now()
	for range tasks {
		ch := Submit(func() (string, error) {
			time.Sleep(work)
			return "done", nil
		})
		<-ch // 提交完立刻取：把并行任务做成了串行
	}
	return time.Since(start)
}

// GetSerializationRight 正确示范：先全部提交，最后统一 get。
// 总耗时 ≈ 单个任务耗时（Java 版 ~1000ms vs 错法 ~2000ms）。
func GetSerializationRight(work time.Duration, tasks int) time.Duration {
	start := time.Now()
	chs := make([]<-chan Result, tasks)
	for i := range chs {
		chs[i] = Submit(func() (string, error) {
			time.Sleep(work)
			return "done", nil
		})
	}
	for _, ch := range chs {
		<-ch
	}
	return time.Since(start)
}

// PoolStarvation 池饥饿死锁 —— 四大坑里的最重的一个（Pool Induced Deadlock）。
//
// 场景：固定 worker 池里的任务，内部又向【同一个池】提交子任务并阻塞等待其结果。
// 2 个 worker 各抱着一个外层任务在等子任务 → 子任务在队列里排队 → 永远没有空闲 worker。
//
// Java md 原话："避免嵌套等待 + 合理池大小"。Go 版同构，且给出了更结构化的两条出路：
//  1. 把池调大（本函数的 fixed=false 用于演示死锁，PoolStarvationFixed 演示调大即可解）；
//  2. 生产级方案：子任务不等结果，改为"完成后回调/写 channel"，外层任务不占 worker 睡觉；
//  3. 或者干脆不用池 —— Go 的默认姿势是 per-task goroutine，这个坑在无池代码里不存在。
//
// 返回是否在 deadline 内死锁（broken 版必然 true；主 goroutine 靠 select+timer 存活，
// 所以不会触发 Go runtime 的"all goroutines asleep"panic —— 那个 panic 只在
// 全部 goroutine 都永久阻塞时才发生）。
func PoolStarvation(deadline time.Duration, workers int) (deadlocked bool) {
	pool := NewInnerPool(workers)
	var completed atomicCounter

	// 3 个【池内】外层任务：每个都向同一个池提交子任务并阻塞等结果。
	for range 3 {
		pool.Submit(func() int {
			inner := pool.Submit(func() int { return 1 }) // 子任务进同一个池
			<-inner                                       // worker 在这里睡觉等子任务 —— 死锁点
			completed.Add(1)
			return 0
		})
	}

	deadlineHit := time.After(deadline)
	for completed.Load() < 3 {
		select {
		case <-deadlineHit:
			return completed.Load() < 3 // 期限内没全部完成 = 死锁
		case <-time.After(5 * time.Millisecond):
		}
	}
	return false
}

// atomicCounter 极简原子计数（教学演示用）。
type atomicCounter = atomic.Int64

// innerPool 池饥饿演示用的极简池（workers 个 worker + 有界队列）。
type innerPool struct {
	jobs chan func()
}

// NewInnerPool 起 workers 个 worker。
func NewInnerPool(workers int) *innerPool {
	p := &innerPool{jobs: make(chan func(), 16)}
	for range workers {
		go func() {
			for job := range p.jobs {
				job()
			}
		}()
	}
	return p
}

// Submit 向池里追加任务，返回结果 channel（教学演示专用）。
func (p *innerPool) Submit(f func() int) <-chan int {
	ch := make(chan int, 1)
	p.jobs <- func() { ch <- f() }
	return ch
}
