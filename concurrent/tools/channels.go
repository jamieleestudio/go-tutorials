package tools

import "sync"

// 三个"channel 天然对应"的 j.u.c. 工具 —— 对应 Java ExchangerExample、
// SynchronousQueueExample、BlockingQueuePipelineExample。
//
// 共同的教学点：Java 需要专门类才能表达的三种交接模式，Go 用 channel 的三个形态
// 直接表达：
//
//	Exchanger        → 两条方向相反的无缓冲 channel（A 给 B 的、B 给 A 的）
//	SynchronousQueue → 一条无缓冲 channel（发送阻塞到接收方接住为止）
//	BlockingQueue    → 有界 channel + 毒丸终止（poison pill）

// ExchangerDemo 对应 ExchangerExample：两个 goroutine 互换彼此的值。
// t1 给出 "A" 拿到 "B"；t2 给出 "B" 拿到 "A"。
func ExchangerDemo() (t1Got, t2Got string) {
	toT1 := make(chan string) // B → t1
	toT2 := make(chan string) // A → t2
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { // t1
		defer wg.Done()
		toT2 <- "A" // 交出自己的值（阻塞到 t2 接住）
		t1Got = <-toT1
	}()
	go func() { // t2
		defer wg.Done()
		t2Got = <-toT2
		toT1 <- "B"
	}()
	wg.Wait()
	return t1Got, t2Got // "B", "A"
}

// SynchronousQueueDemo 对应 SynchronousQueueExample：零缓冲交接，put 阻塞到 take 为止。
// 返回事件顺序日志。关键断言（测试里）：got-X 必然先于 sent-Y —— 生产者发 Y 之前
// 消费者必须已经拿走 X（零缓冲的结构性保证）。
func SynchronousQueueDemo() []string {
	q := make(chan string) // 无缓冲
	var (
		mu     sync.Mutex
		events []string
	)
	log := func(e string) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { // producer
		defer wg.Done()
		q <- "X" // 阻塞：直到 consumer 接住
		log("sent-X")
		q <- "Y"
		log("sent-Y")
	}()
	go func() { // consumer
		defer wg.Done()
		v := <-q
		log("got-" + v)
		<-q // 再取第二个
		log("got-Y")
	}()
	wg.Wait()
	return events
}

// PoisonPillPipeline 对应 BlockingQueuePipelineExample：有界队列 + 毒丸终止。
// 生产者写 1..total 后放毒丸 -1；消费者遇毒丸退出循环。
// Java LinkedBlockingQueue(5) 的容量语义 = channel 缓冲 5：队列满时生产者阻塞（背压）。
func PoisonPillPipeline(total int) []int {
	q := make(chan int, 5) // 对应 new LinkedBlockingQueue<>(5)
	consumed := make([]int, 0, total)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { // producer
		defer wg.Done()
		for i := 1; i <= total; i++ {
			q <- i // 队列满时阻塞 —— 背压
		}
		q <- -1 // 毒丸（poison pill）
	}()
	go func() { // consumer
		defer wg.Done()
		for v := range q { // 消费到毒丸为止
			if v == -1 {
				break
			}
			consumed = append(consumed, v)
		}
	}()
	wg.Wait()
	return consumed
}
