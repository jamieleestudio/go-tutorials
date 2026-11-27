//go:build !race

// PlainCounterBroken 丢更新演示 —— 对应 Java VolatileNotAtomicDemo（volatile int counter，
// 8 线程 × 20 万次 ++，输出 expected/actual/lost）。
//
// 这个演示【本身就是数据竞争】，所以它活在两个构建标签后面（本文件 !race，真跑）：
//
//	go test          → 返回真实丢更新结果：got < want（对应 Java 的 lost > 0）
//	go test -race    → 竞争检测器当场报 DATA RACE —— notatomic_race.go 的占位实现会接住测试
//
// 这正是 Go 相对 Java 的诊断优势：Java volatile++ 静默丢更新（输出 lost > 0 才发现问题），
// Go 的 race detector 直接把"哪里竞争、谁读谁写"打印出来。
// 修复方式对应 Java 的 AtomicInteger：Go 用 atomic.AddInt64 或 Mutex（见 notatomic.go）。
package volatile

import "sync"

// PlainCounterBroken 裸共享计数器：goroutines 个 goroutine 各加 iters 次，无任何同步。
func PlainCounterBroken(goroutines, iters int) (got, want int) {
	shared := 0
	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range iters {
				shared++ // 读-改-写三步，无同步 —— 数据竞争（故意的）
			}
		}()
	}
	wg.Wait()
	return shared, goroutines * iters
}
