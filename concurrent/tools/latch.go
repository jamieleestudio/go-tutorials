// Package tools 对应 java-tutorials/concurrent/tools 子模块：j.u.c. 协作工具箱。
//
// Java README 的分类法照搬，每个工具给出 Go 对应物：
//
//	协作同步   CountDownLatch → sync.WaitGroup（defer Done = finally countDown）
//	          CyclicBarrier  → 手写 Barrier（sync.Cond 版，可复用 + 破损传播）
//	          Phaser         → 分阶段 WaitGroup 循环
//	          Exchanger      → 一对反向的无缓冲 channel
//	限流控制   Semaphore      → 缓冲 channel 实现的信号量
//	通信缓冲   BlockingQueue  → 有界 channel + 毒丸终止（poison pill）
//	          SynchronousQueue → 无缓冲 channel（put 阻塞到 take 为止）
//	数据一致性 ReadWriteLock  → sync.RWMutex（读并发、写独占）
//	          AtomicInteger  → sync/atomic（唯一编号分配）
//
// Java README 的 7 条坑逐条落位：栅栏破损（Barrier.Wait 返回错误）、acquire 未 release
// （Go 版 defer Release）、读写锁升级死锁（RWMutex 同样要先放读锁）、容量与背压
// （有界 channel 天然背压）等，注释里点到对应位置。
package tools

import (
	"sync"
	"time"
)

// LatchDemo 对应 CountDownLatchExample：3 个 worker 各自耗时不同，主线程 await 汇合。
//
// 模式点（Java 版注释强调）：countDown() 放在 finally 里保证异常也放行 ——
// Go 版的 defer wg.Done() 就是这个"finally"的语法级形态，忘不掉。
func LatchDemo(workers int, base time.Duration) []string {
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		done []string
	)
	for i := 1; i <= workers; i++ {
		wg.Add(1) // 对应 latch = new CountDownLatch(workers)
		id := i
		go func() {
			defer wg.Done() // 对应 finally { latch.countDown() } —— 异常也放行
			time.Sleep(base * time.Duration(i))
			mu.Lock()
			done = append(done, "worker-"+itoa(id)+" finished")
			mu.Unlock()
		}()
	}
	wg.Wait() // 对应 latch.await()
	return done
}

// PhaserDemo 对应 PhaserExample：3 参与者两阶段推进。
// Java 的 arriveAndAwaitAdvance() ×2 → Go 的"每阶段一个 WaitGroup"循环。
// 返回事件时间线：所有 phase-0 事件必然先于第一个 phase-1 事件（阶段栅栏语义）。
func PhaserDemo(parties, phases int) []string {
	var (
		mu       sync.Mutex
		timeline []string
	)
	for phase := range phases {
		var wg sync.WaitGroup // 每阶段新建：WaitGroup 计数归零即可复用，但分阶段新建更直白
		for i := 1; i <= parties; i++ {
			wg.Add(1)
			id := i
			go func() {
				defer wg.Done()
				mu.Lock()
				timeline = append(timeline, "phase-"+itoa(phase)+"-arrive worker-"+itoa(id))
				mu.Unlock()
			}()
		}
		wg.Wait() // 对应 arriveAndAwaitAdvance：全员到齐才进下一阶段
	}
	return timeline
}

// itoa 小工具（避免为 3 个数字 import strconv）。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
