package executor

import (
	"sync"
	"sync/atomic"
	"time"
)

// 定时任务 —— 对应 Java ScheduledDemo 的三种形态。
//
// Java ScheduledExecutorService 在 Go 里没有一对一的调度器：标准库答案是 time.AfterFunc
// （延迟单次）与 time.Ticker（周期）。教学重点藏在 Ticker 的行为里：
//
//   - Java scheduleAtFixedRate：任务慢了不并发，但"做完立刻开始下一次"（推迟不丢弃）
//   - Java scheduleWithFixedDelay：周期 = 任务耗时 + 固定间隔
//   - Go time.Ticker：tick 写进容量为 1 的 channel —— 任务慢了【直接丢 tick】，
//     既不推迟也不排队。语义比 Java 的两种都更"放弃式"，慢任务场景要自己改用
//     "执行完再 Reset"的循环（见 FixedDelayDemo）。
//
// 三者对照表（任务耗时 > 周期时）：
//
//	scheduleAtFixedRate   → 不并发、推迟（做完立刻开始下一次）
//	scheduleWithFixedDelay → 不并发、固定间隔（做完歇一会儿）
//	time.Ticker           → 不并发、丢 tick（慢任务期间产生的 tick 全部蒸发）

// AfterFuncOnce 对应 schedule(延迟, 单次)：time.AfterFunc 到点回调一次。
// 返回实际延迟（回调在 goroutine 里执行，WaitGroup 等它完成）。
func AfterFuncOnce(delay time.Duration, work time.Duration) (fired bool, elapsed time.Duration) {
	var (
		wg     sync.WaitGroup
		start  = time.Now()
		firedF atomic.Bool
	)
	wg.Add(1)
	time.AfterFunc(delay, func() {
		defer wg.Done()
		time.Sleep(work) // 任务本身耗时
		firedF.Store(true)
	})
	wg.Wait()
	return firedF.Load(), time.Since(start)
}

// TickerSkipDemo 对应 scheduleAtFixedRate 的 Go 形态：Ticker + 慢任务 → 丢 tick。
// period 周期触发，work 耗时超过 period → 观察到的执行次数远小于 wall/period。
// 返回实际执行次数（教学断言用：runs << ideal）。
func TickerSkipDemo(period, work, wall time.Duration) (runs, ideal int) {
	var n atomic.Int64
	deadline := time.Now().Add(wall)
	ticker := time.NewTicker(period)
	defer ticker.Stop()
	go func() {
		time.Sleep(wall)
		ticker.Stop() // 双保险：即使主循环慢了也保证退出
	}()
	for {
		now := time.Now()
		if now.After(deadline) {
			break
		}
		select {
		case <-ticker.C:
			n.Add(1)
			time.Sleep(work) // 慢任务：耗时 > period → 期间 tick 被丢
		default:
			time.Sleep(period / 4) // 避免空转占满 CPU
		}
	}
	ideal = int(wall / period)
	return int(n.Load()), ideal
}

// FixedDelayDemo 对应 scheduleWithFixedDelay：执行完再等固定间隔。
// 用"sleep(work) → timer.After(gap)"实现，周期 = work + gap（Java 语义逐字还原）。
// 返回相邻两次执行起点之间的间隔。
func FixedDelayDemo(work, gap time.Duration, rounds int) (gaps []time.Duration) {
	for range rounds {
		start := time.Now()
		time.Sleep(work) // 任务耗时
		time.Sleep(gap)  // 固定间隔（对应 scheduleWithFixedDelay 的 delay）
		gaps = append(gaps, time.Since(start))
	}
	return gaps
}
