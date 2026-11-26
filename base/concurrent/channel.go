package concurrent

import (
	"sync"
	"time"
)

// Unbuffered 无缓冲 channel：发送与接收必须"同时就绪"才完成一次交接 ——
// 发送方会阻塞到有人来收。它是同步点，不只是队列。
// 对照 Java：SynchronousQueue。
func Unbuffered() string {
	ch := make(chan string)
	go func() {
		ch <- "交接完成" // 阻塞，直到主 goroutine 执行 <-ch
	}()
	return <-ch
}

// Buffered 有缓冲 channel：缓冲没满时发送不阻塞，满了才阻塞。
// 对照 Java：ArrayBlockingQueue。
func Buffered() (length, capacity, received int) {
	ch := make(chan int, 3)
	ch <- 1
	ch <- 2 // 此时缓冲里 2 个，发送不阻塞
	length, capacity = len(ch), cap(ch)
	received = <-ch + <-ch
	return
}

// SelectDefault select + default：非阻塞地"探测"channel —— 有数据就处理，没有立即走 default。
// 对照 Java：queue.poll() 的立即返回语义。
func SelectDefault() (sent, received, emptyProbe bool) {
	ch := make(chan int, 2)
	select {
	case ch <- 1: // 缓冲没满，发送立即成功
		sent = true
	default:
	}
	select {
	case v := <-ch:
		received = v == 1
	default:
	}
	select {
	case <-ch: // 不会发生：channel 已经空了
		emptyProbe = false
	default:
		emptyProbe = true // 空了，立即走 default，绝不阻塞
	}
	return
}

// RangeChannel range 会一直读到 channel 被 close 为止 —— 谁生产谁负责 close。
// 对照 Java：迭代器 hasNext()；Go 用 close(ch) 传达"没有更多数据"。
// 教学点：接收方永远不要 close（会 panic）；只 close 一次。
func RangeChannel() (sum int) {
	ch := make(chan int)
	go func() {
		defer close(ch) // 必须 close，否则消费方的 range 永远等下去（死锁）
		for i := 1; i <= 5; i++ {
			ch <- i
		}
	}()
	for v := range ch { // channel 关闭且取空后循环结束
		sum += v
	}
	return // 1+2+3+4+5 = 15
}

// Pipeline 流水线：gen(生产) → double(加工) → 主 goroutine(消费)。
// 每个 stage 都是"输入 channel → 输出 channel"的小函数，可自由组装。
// done channel 是防泄漏的关键：消费者提前退出时，上游的 select 能感知并退出。
func Pipeline() (sum int) {
	done := make(chan struct{})
	defer close(done)

	gen := func(nums ...int) <-chan int {
		out := make(chan int)
		go func() {
			defer close(out)
			for _, n := range nums {
				select {
				case out <- n:
				case <-done: // 消费者已退出：放弃发送，goroutine 正常结束
					return
				}
			}
		}()
		return out
	}
	double := func(in <-chan int) <-chan int {
		out := make(chan int)
		go func() {
			defer close(out)
			for n := range in {
				out <- n * 2
			}
		}()
		return out
	}

	for v := range double(gen(1, 2)) { // (1+2)×2 = 6
		sum += v
	}
	return
}

// SelectMultiplex select 同时等待多个 channel：谁先就绪处理谁，不必关心顺序。
// 两个 channel 各发两条，主 goroutine 收齐全部 4 条。
// 对照 Java：NIO 的 Selector，但 Go 的 select 是语言级语法。
func SelectMultiplex() []string {
	a := make(chan string)
	b := make(chan string)
	go func() { a <- "A1"; a <- "A2" }()
	go func() { b <- "B1"; b <- "B2" }()

	out := make([]string, 0, 4)
	for range 4 {
		select {
		case v := <-a:
			out = append(out, v)
		case v := <-b:
			out = append(out, v)
		}
	}
	return out // 到达顺序不确定：select 不保证优先级
}

// TimeoutBySelect select + time.After 实现"最多等这么久"。
// 对照 Java：future.get(3, TimeUnit.SECONDS)。生产代码里更推荐 context（见 context.go）。
// 教学点：ch 缓冲为 1 —— 即使超时放弃，发送方也能完成发送而不会永久卡死（防 goroutine 泄漏）；
// 若是无缓冲 channel，超时分支走掉后发送方会永远阻塞。
func TimeoutBySelect(produceAfter, wait time.Duration) bool {
	ch := make(chan struct{}, 1)
	go func() {
		time.Sleep(produceAfter)
		ch <- struct{}{}
	}()
	select {
	case <-ch:
		return true // 按时收到
	case <-time.After(wait): // 超时计时器，只此一次
		return false
	}
}

// ProduceConsume 单向 channel：chan<- 只发、<-chan 只收 ——
// 方向写进类型里，"消费者发消息""生产者收消息"直接编译报错。
// 对照 Java：接口收窄的思路（方法只暴露需要的操作）。
func ProduceConsume() (sum int) {
	nums := make(chan int) // 双向 channel
	done := make(chan struct{})

	go produce(nums, 1, 5) // 只许发送
	go func() {
		defer close(done)
		sum = consume(nums) // 只许接收
	}()

	<-done // 等 consume 把汇总写完
	return // 15
}

func produce(out chan<- int, from, to int) {
	defer close(out)
	for i := from; i <= to; i++ {
		out <- i
	}
}

func consume(in <-chan int) int {
	total := 0
	for v := range in {
		total += v
	}
	return total
}

// WorkerPool 固定数量的 worker 从 jobs channel 领任务、把结果写进 results ——
// channel 当任务队列，goroutine 当线程池。
// 对照 Java：ThreadPoolExecutor + BlockingQueue 的手工版。
func WorkerPool(workers, jobs int) []int {
	jobCh := make(chan int)
	resultCh := make(chan int)

	var wg sync.WaitGroup
	for w := 1; w <= workers; w++ {
		wg.Add(1)
		go func(id int) { // Go 1.22 起循环变量每轮独立；显式传参仍是稳妥习惯
			defer wg.Done()
			for n := range jobCh {
				resultCh <- n * n // 处理任务：算平方
			}
		}(w)
	}

	go func() {
		for i := 1; i <= jobs; i++ {
			jobCh <- i
		}
		close(jobCh) // 任务发完 → worker 的 range 退出
		wg.Wait()    // 等全部 worker 结束
		close(resultCh)
	}()

	var results []int
	for r := range resultCh { // resultCh 关闭后结束
		results = append(results, r)
	}
	return results // 完成顺序不确定
}
