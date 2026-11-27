// Package virtthread 对应 java-tutorials/concurrent/virtual-thread 子模块 ——
// 全模块的压轴章，也是 Go 最有发言权的一章。
//
// Java 21（Loom 项目）终于把"每任务一线程、便宜到不用池化"变成了现实：
// VirtualThreadPoolDemo 的注释原话"虚拟线程按需创建、不池化，因为创建销毁开销极低"。
// 这句话正是 Go 从 2009 年第一天起的设计：goroutine = M:N 调度的轻量执行体。
//
// 所以本章的叙事是"Java 追上了 Go"：
//
//	BasicVirtualThreadDemo（4 种创建方式）      → Create：Go 只有 go func() 一种，
//	                                              且无 ThreadLocal/无池化负担
//	PerformanceComparisonDemo（1 万任务压测）    → Performance：Go 两边都快（见 perf.go）
//	VirtualThreadPoolDemo（"不要池化虚拟线程"）  → 池化 goroutine 反模式 → Performance 的对照组
//	ScopedValueDemo（ThreadLocal 现代替代）      → Scoped：context.WithValue 的取舍（scoped.go）
//
// Java 版没有 pin 问题演示（侦察确认）；Go 平行概念备注：Go 的 GMP 调度在 goroutine
// 阻塞于 syscall 时会把 P 交给其他线程 —— 与 Loom"载体线程卸载阻塞的虚拟线程"异曲同工。
package virtthread

import (
	"sync"
	"sync/atomic"
)

// Create 对应 BasicVirtualThreadDemo 的 4 种创建方式 —— Go 只有一种：go func()。
// Java 的 4 种（ofVirtual().start / unstarted+start / startVirtualThread / Builder 命名）
// 在 Go 里折叠为一个语句 + "想要名字自己起"（对应 Builder 的 name 前缀）。
// 返回全部执行完成的任务数。
func Create(n int) int {
	var (
		wg      sync.WaitGroup
		counter atomic.Int64
	)
	for i := 1; i <= n; i++ {
		wg.Add(1)
		id := i // 唯一的"命名"方式：闭包捕获编号（对应 Java 的 Builder name("my-vthread-", i)）
		go func() {
			defer wg.Done()
			_ = id // goroutine 无名字、无返回值、无 ThreadLocal —— 状态全靠捕获
			counter.Add(1)
		}()
	}
	wg.Wait()
	return int(counter.Load())
}
