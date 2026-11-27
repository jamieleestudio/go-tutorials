// CompletableFuture 的 Go 答案 —— 对应 java-tutorials/future/CompletableFutureDemo 与
// ConcurrentDbQueryDemo。
//
// 对照表：
//
//	supplyAsync + get            → Submit + <-ch
//	thenApply 链                 → 纯函数顺序调用（Go 没有回调式链的必要：同步代码就是最直白的管道）
//	allOf(...).join()            → Group.Wait（本文件手写的 errgroup 骨架）
//	exceptionally / handle       → 每个任务返回 error，Wait 聚合第一个错误
//	orTimeout                    → context.WithTimeout 传给任务
//	completedFuture(cached)      → 已缓冲的 channel：make(chan V, 1) 塞入即"已完成"
//
// 为什么手写 Group：golang.org/x/sync/errgroup 是生产标准答案（约 60 行，含 cancel 传播），
// 手写这 30 行骨架正是本模块的教学目的 —— 看清 allOf + exceptionally 的本质就是
// WaitGroup + sync.Once（error 只记第一个）。生产项目请直接用 errgroup。
package future

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Group 手写 errgroup 骨架（对应 allOf + exceptionally）。
type Group struct {
	wg      sync.WaitGroup
	errOnce sync.Once
	err     error
}

// Go 提交一个并发任务；任一任务返回非 nil error 时，Wait 只返回第一个
// （对应 Java allOf 后 get 抛出的第一个 ExecutionException）。
func (g *Group) Go(f func() error) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		if err := f(); err != nil {
			g.errOnce.Do(func() { g.err = err }) // 只记第一个错误
		}
	}()
}

// Wait 等全部任务结束；有错误则返回第一个（对应 allOf().join() 的异常路径）。
func (g *Group) Wait() error {
	g.wg.Wait()
	return g.err
}

// ChainDemo 对应 CompletableFutureDemo 的 thenApply 链：订单 → 金额 → 发票。
// Go 版就是三个同步函数 —— 链式回调是"异步世界补可读性"的工具，
// Go 的同步风格天然可读，这本身就是教学点。
func ChainDemo() string {
	order := "Order_123"
	amount := amountFor(order)           // thenApply #1
	invoice := invoiceFor(order, amount) // thenApply #2
	return fmt.Sprintf("%s -> Amount: $%s -> Invoice: %s", order, amount, invoice)
}

func amountFor(order string) string          { return "100" }
func invoiceFor(order, amount string) string { return "Created" }

// AllOfCached 对应 completedFuture(cached)：已完成态的 Future 就是缓冲 1 的 channel。
func AllOfCached() (done bool, value string) {
	ch := make(chan string, 1)
	ch <- "Cached Value (Immediate)" // 预填 = completedFuture
	select {
	case v := <-ch:
		return true, v // select 立即命中 —— "get 不再阻塞"
	default:
		return false, ""
	}
}

// QueryResult 单条"SQL"的结果（对应 ConcurrentDbQueryDemo 的 Result）。
type QueryResult struct {
	SQL  string
	Data string
	Err  error
}

// ParallelQueries 对应 ConcurrentDbQueryDemo：3 条 SQL 并行查，1 条必然失败，
// 每条带自己的超时（对应 orTimeout），handle 语义聚合成功/失败。
func ParallelQueries(ctx context.Context) (success, failed int, firstErr error) {
	queries := []struct {
		sql     string
		latency time.Duration
		fail    bool
	}{
		{sql: "SELECT * FROM users", latency: 30 * time.Millisecond},
		{sql: "SELECT count(*) FROM orders", latency: 50 * time.Millisecond, fail: true},
		{sql: "SELECT * FROM payments", latency: 70 * time.Millisecond},
	}

	var okCount, failCount atomic.Int64
	g := &Group{}
	for _, q := range queries {
		q := q
		g.Go(func() error {
			taskCtx, cancel := context.WithTimeout(ctx, q.latency+200*time.Millisecond) // orTimeout 对应物
			defer cancel()
			select {
			case <-time.After(q.latency): // 模拟查询耗时
				if q.fail {
					failCount.Add(1)
					return fmt.Errorf("db error (sql=%s)", q.sql)
				}
				okCount.Add(1)
				return nil
			case <-taskCtx.Done():
				failCount.Add(1)
				return fmt.Errorf("timeout (sql=%s)", q.sql)
			}
		})
	}
	firstErr = g.Wait()
	success = int(okCount.Load())
	failed = int(failCount.Load())
	return success, failed, firstErr
}
