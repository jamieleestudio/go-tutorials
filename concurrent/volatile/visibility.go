// Package volatile 对应 java-tutorials/concurrent/volatile 子模块。
//
// Java volatile README 的"只要记住 3 句话"是最好的蓝本：
//  1. volatile 解决"看不见"和"乱序"，不解决"同时改"；
//  2. volatile 读 = acquire，volatile 写 = release；
//  3. 出现"先读再写 / ++ / check-then-act / 复合条件"就不是 volatile 能单独扛住的。
//
// Go 没有 volatile 关键字，也没有对应物 —— 因为 Java 靠它解决的问题在 Go 里拆给了
// 不同的答案：
//
//	可见性 + 顺序  → sync/atomic（Go 的 atomic 默认 seq-cst 顺序，读 acquire 写 release）
//	同时改        → sync/atomic 的 Add/CAS 或 sync.Mutex
//	跨 goroutine 发布 → channel（发送 happens-before 对应接收）或 atomic.Pointer
//	数据竞争本身   → race detector（go test -race）—— Java 没有的免费防线
//
// Java 版的 5 个演示类对应关系：
//
//	ReOrder                           → happensbefore.go（契约版，不可确定性复现的原因见该文件）
//	VolatileVisibilityDemo            → visibility.go
//	VolatileNotAtomicDemo             → notatomic.go（丢更新 + build tag 的故事）
//	VolatileDclSingletonDemo          → singleton.go（与 mutex/dcl.go 互相印证）
//	VolatileReferencePublicationDemo  → snapshot.go（不可变快照 + atomic.Pointer）
package volatile

import (
	"sync/atomic"
	"time"
)

// StopFlagAtomic 对应 VolatileVisibilityDemo 的 case2：volatile 停止标记 → atomic.Bool。
//
// Java case1 的坑（普通 boolean + JIT 把 flag 提升进寄存器 → worker 永远空转）在 Go 里
// 变成了更明确的东西：裸 bool 跨 goroutine 一写一读【就是数据竞争】，`go test -race`
// 会当场报 DATA RACE 而不是静默出错 —— 这比 Java 的"可能永远转下去"诊断性好得多。
// 所以本包不写裸 bool 版的运行代码（写了 race 测试就挂），教训放进注释。
func StopFlagAtomic(work, stopAfter time.Duration) (spins int64, stoppedInTime bool) {
	var flag atomic.Bool
	flag.Store(true)
	var spinsCount atomic.Int64
	done := make(chan struct{})
	go func() {
		defer close(done)
		for flag.Load() { // 对应 Java while(running) 空转
			spinsCount.Add(1)
		}
	}()
	time.Sleep(stopAfter) // 对应 Java 主线程 200ms 后置 false
	flag.Store(false)     // 对应 volatile 写（release）
	_ = work
	select {
	case <-done:
		stoppedInTime = true // worker 观察到了新值（acquire）
	case <-time.After(2 * time.Second):
	}
	return spinsCount.Load(), stoppedInTime
}

// StopFlagChannel 是同一需求的 Go 惯用形态：close(done) 广播停止。
// channel 的 close happens-before 所有后续接收 —— 可见性由内存模型契约保证，
// 连 atomic 都不需要。Java 版没有这个第三选项（join/interrupt 是它的等价物）。
func StopFlagChannel(work, stopAfter time.Duration) bool {
	done := make(chan struct{})
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		for { // 对应 while(running)
			select {
			case <-done: // 观察到广播 → 停
				return
			default:
			}
			_ = work
		}
	}()
	time.Sleep(stopAfter)
	close(done)
	select {
	case <-workerDone:
		return true
	case <-time.After(2 * time.Second):
		return false
	}
}
