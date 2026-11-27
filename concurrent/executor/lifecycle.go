package executor

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// 优雅关闭 —— 对应 Java LifecycleDemo 的 shutdown 三步曲：
//
//	shutdown()        → 不再接新任务，存量任务继续跑完
//	awaitTermination  → 带超时地等
//	shutdownNow()     → 超时后强制：中断在跑的任务，返回队列里被丢弃的任务列表
//
// Go 版对应物由三件事拼成：
//
//	close(jobs)           = shutdown（channel 关闭即"不再接新任务"，且 range 自然排空存量）
//	wg.Wait / select 超时 = awaitTermination(timeout)
//	context cancel        = shutdownNow 的中断信号（worker 里 select ctx.Done() 自行退出）
//	                      + 被丢弃任务 = 已提交未完成的计数差（对应 shutdownNow 的返回值）

// GracefulDrain 对应 shutdown + awaitTermination 全员跑完的路径：
// 提交 total 个任务后立刻 close 队列 —— worker 会把缓冲里排队的存量任务全部排空。
// 这就是"close 即 shutdown"：关闭的 channel 依然可以被读空。
func GracefulDrain(total, workers int, work time.Duration) (completed int) {
	p := NewPool(workers, total)
	var done atomic.Int64
	for range total {
		p.Submit(Job{Run: func() {
			time.Sleep(work)
			done.Add(1)
		}})
	}
	p.Wait() // close + 排空存量
	return int(done.Load())
}

// ForceShutdown 对应 LifecycleDemo 的 shutdownNow 路径 —— 确定性握手编排版：
// workers 个 worker，提交 total 个任务（前 workers 个"快任务"= work 后完成，
// 接着 workers 个"长任务"= 卡在 gate 上模拟慢 IO，其余排队）。
// 握手确认 2×workers 个任务已被取走后 cancel —— 状态被钉死：快任务全部完成，
// 长任务被中断退出且【绝不回来取任务】，队列里剩下的从此无人认领 = 被丢弃。
//
// 关键语义：shutdownNow 的 worker 不再从队列取任务 —— 如果 cancel 后继续排空队列
// （range jobs 会一路读完），"丢弃"就永远不发生。这就是为什么教学版 worker 的
// 取任务也要 select ctx.Done，且被中断时直接 return 退出 worker。
//
// 为什么不用"deadline vs 任务耗时"的时序版：Go select 在多个 case 就绪时随机选择，
// cancel 瞬间若 worker 恰好停在取任务的 select 上，可能随机抽到队列继续拿任务 ——
// 时序概率演示换成交接编排的确定性复现（本模块的设计哲学）。
func ForceShutdown(total, workers int, work time.Duration) (completed, dropped int) {
	jobs := make(chan Job, total)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gate := make(chan struct{})            // 长任务闸门：模拟卡死的慢任务（永不放行）
	pickedCh := make(chan struct{}, total) // 握手：worker 每取走一个任务报告一次
	var doneCount atomic.Int64
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() { // 教学版 worker：取任务与执行都要响应"中断"
			defer wg.Done()
			for {
				select {
				case <-ctx.Done(): // shutdownNow：不再接新任务
					return
				case _, ok := <-jobs: // 取任务也要 select：cancel 后立即停手
					if !ok {
						return
					}
					pickedCh <- struct{}{} // 报告：已取走一个
					select {
					case <-time.After(work): // 快任务：正常完成
						doneCount.Add(1)
					case <-gate: // 长任务：卡住（对应 Java 任务卡在慢 IO 上）
					case <-ctx.Done(): // 被"中断"（对应 Java 任务响应 interrupt）
						return // 直接退出 worker —— 绝不回来取任务
					}
				}
			}
		}()
	}

	for range total {
		jobs <- Job{}
	}
	close(jobs) // shutdown：不再接新提交（提交方视角）

	// 握手钉死状态：等 workers 个快任务 + workers 个长任务被取走（channel FIFO，
	// 快任务必然先被取走；worker 取长任务前必已完成自己的快任务）。
	// 此刻：快任务全部完成、长任务全部卡在 gate 上、其余任务在队列里。
	for range 2 * workers {
		<-pickedCh
	}
	cancel() // shutdownNow：中断在跑的 + 弃掉队列里没人取的
	wg.Wait()

	completed = int(doneCount.Load()) // = workers：快任务全部完成
	dropped = total - 2*workers       // 从未有人取走 = shutdownNow 返回的队列残留
	return completed, dropped
}

// Submitter 带"shutdown 后提交"保护的提交器。
//
// 教学点：Java 对已 shutdown 的池 submit 会拿到 RejectedExecutionException；
// Go 对已关闭的 channel 发送是【panic】—— 更痛，且 select/default 救不了
// （向已关闭 channel 发送无论是否在 select 里都会崩）。
// 正确答案不是 recover，而是所有权设计：提交与关闭用同一把锁互斥 ——
// 锁保护的不只是 closed 标志，而是"close 动作与 send 动作永不同时发生"。
type Submitter struct {
	mu     sync.Mutex
	closed bool
	ch     chan int
}

// NewSubmitter 建提交器（队列容量 capacity）。
func NewSubmitter(capacity int) *Submitter {
	return &Submitter{ch: make(chan int, capacity)}
}

// Submit 关闭后返回 false（对应 RejectedExecutionException 的布尔形态）。
func (s *Submitter) Submit(v int) bool {
	s.mu.Lock()
	defer s.mu.Unlock() // 锁全程持有：发送期间 Close 无法插入
	if s.closed {
		return false
	}
	select {
	case s.ch <- v:
		return true
	default:
		return false
	}
}

// Close 关闭队列（与 Submit 互斥，保证不向已关闭的 channel 发送）。
func (s *Submitter) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		close(s.ch)
	}
}

// SubmitAfterClose 演示：shutdown 后再提交 → 被拒绝而非 panic。
func SubmitAfterClose() (rejected bool) {
	s := NewSubmitter(4)
	s.Close()
	return !s.Submit(1)
}
