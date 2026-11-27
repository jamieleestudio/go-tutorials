package volatile

import "sync"

// 对应 Java VolatileDclSingletonDemo：16 线程并发抢单例，DCL + volatile instance。
//
// Java 版的两个关键点：
//  1. instance 字段必须 volatile —— 防止 new 的三步（分配/构造/赋引用）重排被观察到；
//  2. 先读局部变量 local = instance 再判 —— 避免重复读 volatile（性能细节）。
//
// Go 版视角：DCL 的锁那一半在 mutex/dcl.go 已讲透（裸→加锁→双检→sync.Once），
// 本文件补"volatile 那一半"的 Go 答案：
//   - sync.Once 内部正是 mutex + atomic 的组合，等价于"DCL 写对了的样子"；
//   - 若坚持手写 DCL，instance 必须用 atomic.Pointer[T]，裸指针赋值是数据竞争；
//   - Java 的局部变量优化对应 Go 的"Load 一次存局部变量"，同样是避免重复 atomic 读。
//
// OnceValueDCL：32 个 goroutine 并发调用，构造只执行 1 次（用计数器断言），
// 所有调用者拿到同一个实例 —— 对应 Java 版 main 的 done 输出（无 unexpected value 行）。
func OnceValueDCL(callers int) (constructions int, sameValue bool) {
	var (
		once sync.Once
		inst *singletonLike
	)
	var mu sync.Mutex
	constructionCount := 0

	// loadInstance 对应 Java 版的 "local = instance" 局部变量读法。
	loadInstance := func() *singletonLike {
		once.Do(func() {
			inst = &singletonLike{value: 42}
			mu.Lock()
			constructionCount++
			mu.Unlock()
		})
		return inst // Once 保证：Do 返回后 inst 对所有 goroutine 可见（内部有 happens-before）
	}

	results := make(chan int, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- loadInstance().value
		}()
	}
	wg.Wait()
	close(results)

	sameValue = true
	first := 0
	for v := range results {
		if first == 0 {
			first = v
		} else if v != first {
			sameValue = false
		}
	}
	return constructionCount, sameValue
}

// singletonLike 独立于 mutex 包的最小单例对象（避免跨包引用）。
type singletonLike struct {
	value int
}
