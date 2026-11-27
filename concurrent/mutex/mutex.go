// Package mutex 对应 java-tutorials/concurrent/sychronized 子模块（Java 目录名拼写就是 sychronized）。
//
// Java synchronized 的三重语义（互斥 + 可见性 + 有序性）在 Go 里的对应物是 sync.Mutex
// —— 注意 Go 的答案少了一半：Mutex 只保证互斥，可见性/有序性由内存模型 + happens-before
// 契约兜底（见 volatile 包）。本包逐条对照 Java 版的四个演示类：
//
//	SynchronizedMethod        → InstanceLock（实例锁 vs 锁错对象，破损版见 broken.go）
//	SynchronizedStaticMethod  → 类锁 = Class 对象 → Go 里包级锁/结构体级锁的选择
//	SynchronizedInline        → 同步代码块 = Lock/Unlock 手动圈定临界区
//	SynchronizedDoubleCheck   → dcl.go 的 DCL 演进 → sync.Once（演进 1-3 的破损版见 broken.go）
//
// 加一个 Java 没有的 Go 特有陷阱：sync.Mutex 是值类型，结构体按值复制后锁也跟着复制
// 成两把 —— 相当于 Java 版"锁对象选错"的编译期版本（go vet 的 copylocks 检查会拦截）。
//
// 破损演示（InstanceLockBroken / BareSingletonBroken / LockedNoSecondCheckBroken /
// DCLCorrect）集中在 broken.go：它们的教学内容【本身就是数据竞争】，race 构建下
// 由 broken_race.go 占位接管 —— 见该文件头部说明。
package mutex

import (
	"fmt"
	"sync"
)

// counter 演示用的最小计数器。
type counter struct {
	mu sync.Mutex
	n  int
}

// InstanceLockCorrect 对应 Java SynchronizedMethod.executeSingleInstance：
// 全部任务共享同一实例（同一把锁）→ 互斥生效，结果分毫不差。
// 对应 Java 的静态 synchronized 方法：锁收敛到"每类唯一"（Class 对象）——
// Go 里等价物是包级锁或共享结构体里的锁，同样是"一把锁一份状态"。
func InstanceLockCorrect(goroutines, iters int) int {
	c := &counter{} // 唯一实例 = 唯一锁（对应 Java 的单实例提交）
	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range iters {
				c.mu.Lock()
				c.n++
				c.mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return c.n
}

// Counters 演示 Go 特有的"锁复制"陷阱的结构体。
// Java 版 SynchronizedInline 的坑（锁对象选错）在 Go 有编译期防线 —— 但防线挡不住
// 你手动制造副本。sync.Mutex 是值类型：含锁结构体按值传递/复制后，锁和数据一起复制，
// 两份副本互不知晓。
//
// 下面的代码会被 go vet 的 copylocks 检查直接拒绝（这也是为什么本包不能真的写它）：
//
//	func (c Counters) IncBroken() { c.mu.Lock(); c.n++; c.mu.Unlock() }
//	// go vet: IncBroken passes lock by value: mutex.Counters contains sync.Mutex
//	//         （值接收者 = 递进来的锁是副本，n 也是副本 —— 加了等于白加）
//
// 规则：含 sync.Mutex（或 WaitGroup/Cond/Once/atomic.Value 等）的类型一律指针接收者、
// 一律不按值拷贝。vet 是免费的保险丝，CI 里必须跑。
type Counters struct {
	mu sync.Mutex
	n  int
}

// Inc 正确姿势：指针接收者，所有调用者共享同一把锁、同一份数据
// （对应 Java SynchronizedInline：synchronized (lock) { c++ } 圈定同一临界区）。
func (c *Counters) Inc(iters int) {
	for range iters {
		c.mu.Lock()
		c.n++
		c.mu.Unlock()
	}
}

// Value 读当前值（同样指针接收者；读也要过锁，否则是数据竞争）。
func (c *Counters) Value() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

// GuardedBlock 对应 Java SynchronizedInline：显式锁对象 + 手动圈定临界区。
// Java 版附带的第二个坑（executor.shutdown() 后忙等 isTerminated）在 Go 里
// 被 WaitGroup 天然解决 —— 见 tools 包的 LatchDemo。
func GuardedBlock(goroutines, iters int) int {
	var mu sync.Mutex // 显式锁对象（对应 Java 的 public static final Object lock）
	shared := 0
	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range iters {
				mu.Lock()   // synchronized (lock) {
				shared++    //   c++;
				mu.Unlock() // }
			}
		}()
	}
	wg.Wait()
	return shared
}

// String 组装演示输出（对应 Java 版 main 的打印行）。
func String() string {
	got, want := InstanceLockBroken(16, 1000)
	s := fmt.Sprintf("InstanceLockBroken: got=%d want=%d (lost=%d)\n", got, want, want-got)
	s += fmt.Sprintf("InstanceLockCorrect: n=%d (want %d)\n", InstanceLockCorrect(16, 1000), 16000)
	c := &Counters{}
	c.Inc(1000)
	s += fmt.Sprintf("GuardedBlock/pointer receiver: n=%d\n", c.Value())
	return s
}
