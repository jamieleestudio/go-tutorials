//go:build !race

// 破损演示合集 —— 本文件的每个函数【本身就是数据竞争】，而数据竞争正是它们的教学内容。
// 所以整个文件活在 //go:build !race 标签后面：
//
//	go test        → 真跑：丢更新/重复创建如期发生（通道编排确定性复现）
//	go test -race  → 不能真跑 —— race detector 会当场报 DATA RACE，测试直接失败；
//	                 此时 broken_race.go 的占位实现接管，返回教学叙事期望的形状。
//
// 这个二选一本身就是本模块的核心论点之一：Java 版这些坑要靠"输出 lost > 0 /
// 输出两次创建"才能发现问题（静默出错），Go 的 race detector 在你写代码的当下
// 就把竞争双方点名 —— 诊断性天差地别。修复版全部 race-clean：InstanceLockCorrect、
// GuardedBlock（mutex.go）与 OnceSingleton（dcl.go）。
package mutex

import "sync"

// InstanceLockBroken 对应 Java SynchronizedMethod.executeMultipleInstance 的坑：
// 每个任务 new 一个实例 → 每个实例自带一把锁 → 100 把锁各锁各的，共享状态裸奔。
//
// Java 版输出 c < 100（丢更新）；Go 版同构：goroutine 各持一把新锁去"保护"共享变量，
// 结果必然丢更新。锁必须和被保护的数据绑定在同一个对象上 —— 这是所有锁语义的第一课。
func InstanceLockBroken(goroutines, iters int) (got, want int) {
	shared := 0
	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		each := &counter{} // 每个任务一把新锁 —— 对应 Java 的 new SynchronizedMethod().increase()
		go func() {
			defer wg.Done()
			for range iters {
				each.mu.Lock()
				shared++ // 锁的是 each，动的是 shared：互斥彻底失效（数据竞争，故意的）
				each.mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return shared, goroutines * iters
}

// BareSingletonBroken 演进 1：裸单例无锁。
// 通道编排让两个 goroutine 都先通过第一次判空、再各自赋值 —— 确定性创建两次。
// （Java 版是概率复现：并发压力下撞上同一窗口；Go 版用握手通道把窗口钉死。）
func BareSingletonBroken() int {
	var inst *singleton
	checked := make(chan struct{}) // 报告"已通过判空"
	release := make(chan struct{}) // 主线程放行创建（保证两人都过判空后才开始）
	creations := make(chan *singleton, 4)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if inst == nil { // 第一次判空（无任何同步）—— 裸读，数据竞争
				checked <- struct{}{} // 两人都到这里：窗口钉死
				<-release             // 等放行 —— 期间 inst 仍是 nil
				created := makeSingleton()
				inst = created // 裸写，与对方的裸读/裸写竞争
				creations <- created
			}
		}()
	}
	<-checked
	<-checked // 两个 goroutine 都已通过判空
	release <- struct{}{}
	release <- struct{}{}
	wg.Wait()
	close(creations)
	n := 0
	for range creations {
		n++
	}
	return n // 2：同一个单例被造了两次
}

// LockedNoSecondCheckBroken 演进 2：加锁但不做第二次判空。
// md 笔记的核心推演：两个线程排队过锁，第二个进锁时不知道第一个已经建好。
// 注意：锁内的写本身没问题，竞争发生在【锁外的第一次判空】与【锁内写】之间。
func LockedNoSecondCheckBroken() int {
	var inst *singleton
	var mu sync.Mutex
	checked := make(chan struct{})
	release := make(chan struct{})
	creations := make(chan *singleton, 4)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if inst == nil { // 裸读 —— 与另一个 goroutine 锁内的写竞争
				checked <- struct{}{} // 通过第一次判空
				<-release             // 两人都过判空后，先后进锁
				mu.Lock()
				inst = makeSingleton() // 锁内【没有】再判空 → 第二个照建不误
				mu.Unlock()
				creations <- inst
			}
		}()
	}
	<-checked
	<-checked
	release <- struct{}{}
	release <- struct{}{}
	wg.Wait()
	close(creations)
	n := 0
	for range creations {
		n++
	}
	return n // 2：锁保证了互斥，但没保证"只建一次"
}

// DCLCorrect 演进 3：双重判空 + 锁 —— 对应 Java SynchronizedDoubleCheck 的写法。
// 锁外判空（快速路径）+ 锁内再判空（防排队重复建）→ 只创建一次。
//
// 函数名里的 Correct 是【Java 语境】的正确（前提：instance 声明 volatile）；
// Go 里锁外的第一次判空仍是裸读 vs 锁内写的竞争 —— -race 会点名，名字保留 Correct
// 是为了对照"Java volatile 能救、Go 裸读救不了"的语义差。Go 的正确答案见 dcl.go 的
// OnceSingleton（或手写版：atomic.Pointer 快速路径 + mutex 慢速路径）。
func DCLCorrect() (n int, value int) {
	var inst *singleton
	var mu sync.Mutex
	creations := make(chan *singleton, 4)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if inst == nil { // 第一次判空：裸读 —— Go 里这就是竞争点
				mu.Lock()
				if inst == nil { // 第二次判空：进锁后重新确认
					created := makeSingleton()
					inst = created
					creations <- created
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	close(creations)
	n = 0
	for c := range creations {
		n++
		value = c.value
	}
	return n, value // 1：DCL 生效（语义上只建一次）
}
