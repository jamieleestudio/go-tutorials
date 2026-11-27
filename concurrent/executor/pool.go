// Package executor 对应 java-tutorials/concurrent/executor 子模块：线程池 = 任务提交与执行解耦。
//
// Java 版四个演示类的对应关系：
//
//	BasicPoolDemo        → pool.go（Fixed/Single/Cached 三形态 → worker 数与队列参数的选择）
//	CustomThreadPoolDemo → pool.go（七参数对照表 + CallerRunsPolicy → SubmitCallerRuns）
//	ScheduledDemo        → scheduled.go（scheduleAtFixedRate/WithFixedDelay → Ticker 与丢 tick）
//	LifecycleDemo        → lifecycle.go（shutdown/awaitTermination/shutdownNow → drain/force）
//
// Java 版笔记（EXECUTOR_LEARNING.md）最重要的两条论点，Go 版照搬并给出对应物：
//   - "Executors 工厂的风险"：Fixed 用无界 LinkedBlockingQueue 堆积 OOM、Cached 线程爆炸。
//     Go 对应：有界 channel 满了 Submit 立即感知（AbortPolicy 形态）反而安全；真正危险的是
//     【无上限地 go func】—— 等价于 CachedThreadPool 的 Integer.MAX_VALUE 线程爆炸。
//     所以 Go 版的任务池 = 固定 worker + 有界队列，这在 Go 里不是"最佳实践"而是常识。
//   - "submit() 的异常不会打印，必须 future.get() 或任务内 try-catch"：Go 对应
//     goroutine 里被丢弃的 error —— 无人读的结果 channel 就是 Java 版无人 get 的 Future。
package executor

import (
	"sync"
	"sync/atomic"
	"time"
)

// Job 池里跑的任务（对应 Java Runnable/Callable）。
type Job struct {
	ID  int
	Run func()
}

// Pool 固定 worker + 有界队列（对应 ThreadPoolExecutor 的 core=max=workers、queue=capacity）。
// Java 七参数在 Go 的对应关系：
//
//	corePoolSize = maximumPoolSize → workers（固定 worker 数；Go worker 不动态扩缩）
//	keepAliveTime                  → 无对应（worker 常驻到 Wait()；goroutine 创建足够便宜，
//	                                  这正是"要不要 keepAlive"问题的 Go 式消失）
//	workQueue                      → jobs channel 缓冲 = capacity
//	threadFactory                  → worker 编号内置（goroutine 无名字；要名字自己记）
//	handler（拒绝策略）             → Submit 返回 false（Abort 形态）/ SubmitCallerRuns（CallerRuns）
type Pool struct {
	jobs   chan Job
	wg     sync.WaitGroup
	mu     sync.Mutex
	used   map[int]bool // 参与执行的 worker 编号（对应 Java 打印线程名的观察点）
	inline atomic.Int64 // CallerRuns 次数（提交方亲自执行）
}

// NewPool 启动 workers 个常驻 worker，队列缓冲 capacity。
// 对应 Java：new ThreadPoolExecutor(workers, workers, keepAlive, workQueue, factory, handler)
func NewPool(workers, capacity int) *Pool {
	p := &Pool{
		jobs: make(chan Job, capacity),
		used: map[int]bool{},
	}
	for i := 1; i <= workers; i++ {
		p.wg.Add(1)
		id := i
		go func() { // 每个 worker = Java 池里的常驻线程
			defer p.wg.Done()
			for job := range p.jobs { // 无任务时阻塞在 jobs 上（等价池里 idle 线程）
				p.mu.Lock()
				p.used[id] = true
				p.mu.Unlock()
				job.Run()
			}
		}()
	}
	return p
}

// Submit 满时返回 false（对应 AbortPolicy：拒绝并让提交方知道）。
// 与 Java 的区别：Go 的 channel 有界是常态；Java 教学反复强调"手动给队列上界"，
// Go 从语言层面就只有"有界或阻塞"两个选项。
func (p *Pool) Submit(job Job) bool {
	select {
	case p.jobs <- job:
		return true
	default:
		return false // 队列满 → 拒绝
	}
}

// SubmitCallerRuns 对应 CallerRunsPolicy：队列满时提交线程自己执行。
// Java 笔记的原话：CallerRuns = "削峰填谷、减缓提交速度" —— 提交方被拖住，上游自然限流。
// Go 版语义逐字相同：调用方 goroutine 亲自跑 Run()。
func (p *Pool) SubmitCallerRuns(job Job) {
	if p.Submit(job) {
		return
	}
	p.inline.Add(1)
	job.Run()
}

// Wait 对应 shutdown() + awaitTermination()：关队列、等存量任务全部跑完。
// close(jobs) = shutdown（不再接新任务）；wg.Wait = awaitTermination（无限等）。
func (p *Pool) Wait() {
	close(p.jobs)
	p.wg.Wait()
}

// UsedWorkers 参与执行的 worker 数（≤ workers；0 号 = 提交方）。
func (p *Pool) UsedWorkers() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.used)
}

// CallerRuns 返回提交方亲自执行的任务数（对应 Java 输出里的 "main 处理任务 N"）。
func (p *Pool) CallerRuns() int64 { return p.inline.Load() }

// FixedPoolDemo 对应 BasicPoolDemo 的 FixedThreadPool：3 worker 跑 5 任务。
// 返回完成数与参与的 worker 数 —— Java 版打印 pool-1-thread-N（≤3 个不同线程名）。
func FixedPoolDemo(jobs, workers int, work time.Duration) (completed, usedWorkers int) {
	p := NewPool(workers, jobs) // 队列容量 = 任务数：全部排队，无拒绝
	var done atomic.Int64
	for i := 1; i <= jobs; i++ {
		p.Submit(Job{ID: i, Run: func() {
			time.Sleep(work)
			done.Add(1)
		}})
	}
	p.Wait()
	return int(done.Load()), p.UsedWorkers()
}

// SingleWorkerOrder 对应 newSingleThreadExecutor：单 worker 严格按提交顺序执行。
// 返回实际执行顺序 —— Java 版打印"同一线程按 task-1..3 顺序"。
func SingleWorkerOrder(jobs int) []int {
	p := NewPool(1, jobs)
	var order []int
	var mu sync.Mutex
	for i := 1; i <= jobs; i++ {
		p.Submit(Job{ID: i, Run: func() {
			mu.Lock()
			order = append(order, i) // 单 worker 下串行，无需锁也能观察到顺序
			mu.Unlock()
		}})
	}
	p.Wait()
	return order
}

// CallerRunsDemo 对应 CustomThreadPoolDemo 的核心场景：
// 2 worker + 队列容量 2，快速提交 8 个慢任务 → 2 在跑 + 2 排队，其余触发 CallerRuns
// 由提交方（main goroutine）亲自执行 —— Java 版输出里混着 "main 处理任务 N"。
func CallerRunsDemo(total, workers, capacity int, work time.Duration) (completed int, callerRan int64) {
	p := NewPool(workers, capacity)
	var done atomic.Int64
	for i := 1; i <= total; i++ {
		p.SubmitCallerRuns(Job{ID: i, Run: func() {
			time.Sleep(work)
			done.Add(1)
		}})
	}
	p.Wait()
	return int(done.Load()), p.CallerRuns()
}
