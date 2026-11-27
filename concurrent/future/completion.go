package future

import (
	"sync"
	"time"
)

// CompletionService 对应 java-tutorials/callable/CompletionServiceExample：
// 5 个任务耗时递减，ExecutorCompletionService.take() 按完成先后消费
// （Java 输出 done-5, done-4, ..., done-1 —— 与提交顺序相反）。
//
// Go 版不需要 CompletionService 这个类：让所有任务往【同一个】结果 channel 里写，
// channel 天然就是"按完成顺序排队"的容器。这正好呼应 README 的主题：
// Java 的很多框架类（FutureTask、CompletionService、ExecutorCompletionService）
// 在 Go 里是一个 channel 事实就能替代的东西。
func CompletionOrder(work func(i int) time.Duration, n int) []int {
	type done struct {
		id int
	}
	results := make(chan done, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			time.Sleep(work(idx))
			results <- done{id: idx} // 先完成先入队
		}()
	}
	wg.Wait()
	close(results)

	order := make([]int, 0, n)
	for d := range results {
		order = append(order, d.id)
	}
	return order
}

// WebFetchShape 说明性的抓取结果结构（真实实现见 webfetch.go，那里用 httptest 离线复刻）。
type WebFetchShape struct {
	URL       string
	Status    int
	Bytes     int
	Cancelled bool
	Err       error
}
