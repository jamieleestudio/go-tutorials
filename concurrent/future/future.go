// Package future 对应 java-tutorials/concurrent 的 future + callable 两个子模块。
//
// Java 把"任务"（Callable）与"结果句柄"（Future）分成两个模块讲；Go 版合并成一章，
// 因为 goroutine + channel 本来就是一体的：go f() 是提交，<-ch 是 get()。
// 对应关系总表：
//
//	Callable<V> + executor.submit   → ch := make(chan V); go func() { ch <- f() }()
//	Future.get()                    → v := <-ch
//	Future.get(timeout)             → select { case v := <-ch: ...; case <-time.After: ... }
//	Future.cancel(true)             → context.CancelFunc（任务必须主动检查 ctx.Done 才会停）
//	ExecutionException.getCause()   → 直接读 error（Go 的 error 是值，没有包装类；
//	                                  但"无人读的 error"同 Java"无人 get 的 Future"一样被吞）
//	invokeAll(tasks, timeout)       → 全部提交 + 按提交序收集，超时未完成的标记 cancelled
//	CompletionService.take()        → 单个结果 channel：先完成先出（多生产者天然按完成序）
//	FutureTask 直跑线程             → 就是一个 goroutine + channel（不需要"既能 Runnable 又能 Future"的两面派）
//	CompletableFuture 链/allOf      → composition.go（手写 errgroup 骨架）
//
// md 笔记的比喻照搬：Future 就像餐厅小票 —— 点餐（提交）、厨房做（异步）、玩手机（做别的）、
// 凭小票取餐（get）。Go 版：小票就是一个 channel。
package future

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Result 任务结果：值与错误一起走（对应 Java 的"值 or ExecutionException"二选一）。
type Result struct {
	Value string
	Err   error
}

// Submit 最小 Future：返回结果 channel。调用方 <-ch 即 get()，无人读的 channel 被 GC，
// 不会有"线程池泄漏"（goroutine 结束即回收）—— 但无人读的 error 同样被吞（坑见测试）。
func Submit(f func() (string, error)) <-chan Result {
	ch := make(chan Result, 1) // 缓冲 1：任务完成即"存好结果"，无论调用方何时来取
	go func() {
		v, err := f()
		ch <- Result{Value: v, Err: err}
	}()
	return ch
}

// GetWithTimeout 对应 get(timeout)：带超时的取结果。
// Java 版三个分支（中断/ExecutionException/Timeout→cancel(true)）在 Go 折叠成：
// 值/错误/超时三种 select 出口，超时的善后就是取消任务（见 CancelCooperative）。
func GetWithTimeout(ch <-chan Result, timeout time.Duration) (Result, bool) {
	select {
	case r := <-ch:
		return r, true
	case <-time.After(timeout):
		return Result{}, false
	}
}

// Lifecycle 对应 BasicFutureDemo 的叙事线：提交 → 干别的 → 探一下 isDone → 取结果。
// 返回记录的事件序列，测试断言顺序（提交先于"做别的"，结果最后到）。
func Lifecycle() []string {
	var (
		events []string
		mu     sync.Mutex
	)
	log := func(e string) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	}

	ch := Submit(func() (string, error) {
		time.Sleep(30 * time.Millisecond) // 对应 Java 的 2s 耗时计算
		log("task-done")
		return "Hello, Future!", nil
	})
	log("submitted")   // 点餐
	log("doing-other") // 做别的（对应 Java 主线程 sleep 500ms）
	if _, ok := GetWithTimeout(ch, 5*time.Millisecond); !ok {
		log("probe-not-done") // 对应 Java isDone() == false
	}
	r, _ := GetWithTimeout(ch, time.Second)
	log("got:" + r.Value) // 凭小票取餐
	return events
}

// CancelCooperative 对应 FuturePitfallDemo 坑 2：cancel(true) 只是发信号。
//
// Java 版：死循环任务不检查 isInterrupted() → cancel(true) 返回 true 但任务停不下来；
// 循环里加上中断检查 → 正常退出。
// Go 版完全同构：context.CancelFunc 只是关闭 done channel —— 任务必须 select ctx.Done()
// 才会停。uninterruptibleRanToCompletion=true 就是"cancel 返回 true 但任务跑完了"。
func CancelCooperative() (uninterruptibleRanToCompletion, contextAwareStoppedEarly bool) {
	const work = 60 * time.Millisecond

	// 任务 A：不响应取消（对应 Java 不检查 isInterrupted 的死循环）。
	ctxA, cancelA := context.WithCancel(context.Background())
	doneA := make(chan struct{})
	startedA := time.Now()
	go func() {
		defer close(doneA)
		time.Sleep(work) // 只管睡觉：ctx 挂了也不知道
		_ = ctxA
	}()
	time.Sleep(10 * time.Millisecond)
	cancelA() // "cancel(true) 返回 true"—— 但任务 A 照样跑满
	<-doneA
	uninterruptibleRanToCompletion = time.Since(startedA) >= work-10*time.Millisecond

	// 任务 B：协作式取消（对应 Java 循环内检查 isInterrupted）。
	ctxB, cancelB := context.WithCancel(context.Background())
	doneB := make(chan struct{})
	startedB := time.Now()
	go func() {
		defer close(doneB)
		for {
			select {
			case <-ctxB.Done(): // 对应 Java if (Thread.currentThread().isInterrupted()) return;
				return
			case <-time.After(2 * time.Millisecond):
			}
		}
	}()
	time.Sleep(10 * time.Millisecond)
	cancelB()
	select {
	case <-doneB:
		contextAwareStoppedEarly = time.Since(startedB) < work/2
	case <-time.After(time.Second):
	}
	return uninterruptibleRanToCompletion, contextAwareStoppedEarly
}

// InvokeAllOutcome 对应 invokeAll(tasks, timeout) 的语义：
// 返回后按【提交顺序】给出每个任务的结果；到期限还没完成的标记 Cancelled=true
// （Java 版返回的未完成 Future 已被 cancel，要判断 isCancelled）。
type InvokeAllOutcome struct {
	Value     string
	Err       error
	Cancelled bool
}

func InvokeAll(work func(i int) (string, error), n int, timeout time.Duration) []InvokeAllOutcome {
	chs := make([]<-chan Result, n)
	for i := range chs {
		idx := i
		chs[idx] = Submit(func() (string, error) { return work(idx) })
	}
	deadline := time.After(timeout)
	out := make([]InvokeAllOutcome, n)
	for i := range chs { // 按提交顺序收集
		select {
		case r := <-chs[i]:
			out[i] = InvokeAllOutcome{Value: r.Value, Err: r.Err}
		case <-deadline:
			out[i] = InvokeAllOutcome{Cancelled: true} // 期限已过 → 取消态
		}
	}
	return out
}

// ExceptionPropagation 对应 ExceptionPropagationExample：
// Java 的坑是异常被 ExecutionException 包装，必须 getCause() 才见真身；
// Go 的 error 直接是值，但反过来 —— 无人读的 error 静默消失（"被吞"的形式不同，
// 后果相同）。这里的调用方【读】了 error，所以拿得到 boom。
func ExceptionPropagation() error {
	ch := Submit(func() (string, error) {
		return "", errors.New("boom") // 对应 Java throw new IllegalStateException("boom")
	})
	r := <-ch // 若调用方只取 Value 不读 Err，异常同样被吞 —— 坑的形式变了，坑还在
	return r.Err
}
