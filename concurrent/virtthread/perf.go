package virtthread

import (
	"sync"
	"time"
)

// 性能实验 —— 对应 Java PerformanceComparisonDemo 的实验设计，规模等比缩小到 CI 友好：
//
//	Java：10,000 任务 × sleep 1s。平台线程池(100) ≈ 100 秒量级 vs 虚拟线程 ≈ 1 秒多。
//	Go：  1,500 任务 × sleep 40ms。per-task goroutine ≈ 40-200ms vs 50 worker 池 ≈ 1.2 秒。
//
// 结论在 Go 里更彻底：Java 版对照组"100 平台线程池"是必须的（裸建 10,000 个 Thread 会
// OOM）；Go 版对照组只是教学表演 —— per-task goroutine 就是默认姿势，1,500 个甚至
// 100 万个 goroutine 都不需要"池"。VirtualThreadPoolDemo 的"不要池化虚拟线程"在 Go
// 的表述是：**给 goroutine 建池是反模式**（池只在需要限流/背压时才有意义，见 executor 包）。

// PerTaskGoroutines 对应 newVirtualThreadPerTaskExecutor：每任务一个 goroutine，不池化。
// 返回全部完成的总耗时。
func PerTaskGoroutines(n int, work time.Duration) time.Duration {
	start := time.Now()
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() { // 对应 Thread.startVirtualThread / virtual-thread-per-task
			defer wg.Done()
			time.Sleep(work) // 对应 Java 的 Thread.sleep 模拟 IO 阻塞
		}()
	}
	wg.Wait()
	return time.Since(start)
}

// PoolThrottled 对应 newFixedThreadPool(100) 对照组：50 个常驻 worker 分摊任务。
// 返回总耗时 —— 应明显慢于 PerTaskGoroutines（任务在队列里排队等 worker）。
func PoolThrottled(n, workers int, work time.Duration) time.Duration {
	start := time.Now()
	jobs := make(chan struct{}, n)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() { // 常驻 worker
			defer wg.Done()
			for range jobs { // 对应池线程从队列取任务
				time.Sleep(work)
			}
		}()
	}
	for range n {
		jobs <- struct{}{}
	}
	close(jobs)
	wg.Wait()
	return time.Since(start)
}
