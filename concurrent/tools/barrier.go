package tools

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// Barrier 手写 CyclicBarrier —— Go 标准库没有栅栏，用 sync.Cond 实现是最正统的答案
// （Cond 本身就是"等待某个条件成立"的原语，栅栏正是它的教科书场景）。
//
// 对照 Java CyclicBarrier 的三个特性逐条实现：
//  1. 可复用（Cyclic）：全员到达后计数归零、代数(generation)+1，下一轮照常使用；
//  2. 栅栏动作：最后到达者执行 action（对应 Java 构造参数 barrierAction）；
//  3. 栅栏破损（Java README 坑 1）：Cancel 让当前代全员从 Await 拿到
//     ErrBrokenBarrier —— Java 版一线程异常整个栅栏 broken，语义一致。
//
// Java README 坑 7（Phaser 参与者数量注册错误 → await 永远不过）在 Go 版的对应物：
// parties 数与实际 Wait 的 goroutine 数必须一致，否则全员阻塞 —— 测试里用
// 超时守卫演示这个坑的形态。
type Barrier struct {
	mu     sync.Mutex
	cond   *sync.Cond
	n      int    // 参与者数量
	count  int    // 当前代已到达数
	gen    int    // 当前代数（可复用的关键）
	broken bool   // 当前代是否已破损
	action func() // 全员到达时执行（最后到达者负责）
}

// ErrBrokenBarrier 栅栏破损错误（对应 Java BrokenBarrierException）。
var ErrBrokenBarrier = errors.New("broken barrier")

// NewBarrier 建栅栏：n 个参与者，全员到达时执行 action（可为 nil）。
func NewBarrier(n int, action func()) *Barrier {
	b := &Barrier{n: n, action: action}
	b.cond = sync.NewCond(&b.mu)
	return b
}

// Await 到达栅栏并等待全员；返回 nil 表示全员通过，ErrBrokenBarrier 表示本轮作废。
func (b *Barrier) Await() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	gen := b.gen // 记住自己属于哪一代：唤醒后要确认"等的是这一代的结果"
	b.count++
	if b.count == b.n {
		// 最后到达者：放行全员（对应 Java 的 barrierAction + 下一轮重置）
		if b.action != nil {
			b.action()
		}
		b.gen++
		b.count = 0
		b.broken = false
		b.cond.Broadcast()
		return nil
	}

	b.cond.Wait() // 释放锁并睡眠，被 Broadcast 唤醒后重新持锁
	// 唤醒后可能有两种结局：
	//   a) 本代正常放行（gen 已 +1）→ nil
	//   b) 本代被 Cancel 破损（gen 未变，broken=true）→ ErrBrokenBarrier
	if b.broken && b.gen == gen {
		return ErrBrokenBarrier
	}
	return nil
}

// Cancel 打破当前代：所有等待者收到 ErrBrokenBarrier，计数复位。
// 对应 Java：一个参与者的异常会让整轮栅栏 broken。
func (b *Barrier) Cancel() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.broken = true
	b.count = 0
	b.cond.Broadcast()
}

// BarrierRounds 复用演示：3 参与者跑 2 轮，每轮全员到达执行一次 action。
// 返回 action 执行次数（= 轮数）与各轮完成标记。
func BarrierRounds(parties, rounds int) (actions int, completedAllRounds bool) {
	var actionCount atomic.Int64
	done := make(chan struct{})
	b := NewBarrier(parties, func() { actionCount.Add(1) })

	go func() {
		defer close(done)
		var wg sync.WaitGroup
		for range rounds {
			for range parties {
				wg.Add(1)
				go func() {
					defer wg.Done()
					_ = b.Await()
				}()
			}
			wg.Wait() // 本轮全员通过后再发起下一轮（演示复用语义）
		}
	}()

	select {
	case <-done:
		return int(actionCount.Load()), true
	case <-time.After(2 * time.Second):
		return int(actionCount.Load()), false
	}
}

// BarrierBroken 破损演示：3 个参与者，2 个在等待，第 3 个 Cancel → 等待者拿到错误。
// 返回收到 ErrBrokenBarrier 的人数（应为 2）。
func BarrierBroken(parties int) (brokenCount int) {
	b := NewBarrier(parties, nil)
	errs := make(chan error, parties)
	for range parties - 1 { // parties-1 人先到栅栏等待
		go func() {
			errs <- b.Await()
		}()
	}
	time.Sleep(50 * time.Millisecond) // 等等待者就位
	b.Cancel()                        // "第 3 个参与者出异常" → 栅栏破损
	for range parties - 1 {
		if err := <-errs; err == ErrBrokenBarrier {
			brokenCount++
		}
	}
	return brokenCount
}
