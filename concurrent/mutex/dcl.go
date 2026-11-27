// DCL（双重检查锁）单例的演进 —— 对应 java-tutorials 两个模块互相印证的同一主题：
// sychronized/SynchronizedDoubleCheck（锁的写法）与
// volatile/examples/VolatileDclSingletonDemo + 双重检查锁的单例模式.md（为什么要 volatile）。
//
// md 笔记的四步推演，Go 版逐步复现（前 3 步用通道精确编排两个 goroutine，确定性复现而非概率；
// 因为它们【本身就是数据竞争】，真身活在 broken.go 的 //go:build !race 后面）：
//  1. 裸单例：无锁 → 两个线程同时通过判空 → 创建两次        （broken.go）
//  2. 加锁不双检：锁内直接赋值 → 第二个线程进锁后不知道对象已建 → 仍创建两次（broken.go）
//  3. DCL 正确版：双重判空 + 锁 → 只创建一次                 （broken.go —— 见下述"Go 视角"）
//  4. Go 惯用答案：sync.Once（本文件，race-clean）
//
// 关于第 3 步的 Go 视角（重要）：Java 版 DCL 正确的前提是 instance 声明为 volatile；
// Go 里把 volatile 翻译成"atomic 读"而非裸读 —— 但哪怕写对了 Java 式 DCL（锁外判空 +
// 锁内判空 + 锁内赋值），锁外的第一次判空仍是数据竞争（裸读 vs 锁内写），
// go test -race 会当场点名 —— 这正是"Java 用 volatile 解决的问题，Go 必须用
// atomic/Once 解决"的实证。所以本包的最终答案是演进 4 的 sync.Once：
// 内部就是 mutex + atomic 的现成封装，天然 race-clean。
package mutex

import "sync"

// singleton 最小单例对象。
type singleton struct {
	value int
}

// makeSingleton 模拟"构造"（对应 Java new Singleton() 的三步非原子操作）。
func makeSingleton() *singleton {
	return &singleton{value: 42}
}

// OnceSingleton 演进 4：Go 惯用答案 sync.Once。
// Java 版需要 volatile + 双重判空手写的正确性，Go 一个原语封装掉 ——
// 内部实现就是 mutex + atomic 的组合（Do 的 f 未返回前，其他 goroutine 全部阻塞等待）。
// 64 个 goroutine 并发抢单例：只构造一次，所有人拿到同一指针。
func OnceSingleton(callers int) (constructions int, samePointer bool) {
	var (
		once sync.Once
		inst *singleton
	)
	constructionCount := make(chan *singleton, callers)
	results := make(chan *singleton, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			once.Do(func() {
				inst = makeSingleton()
				constructionCount <- inst
			})
			results <- inst
		}()
	}
	wg.Wait()
	close(constructionCount)
	close(results)
	samePointer = true
	var first *singleton
	for r := range results {
		if first == nil {
			first = r
		} else if r != first {
			samePointer = false
		}
	}
	return len(constructionCount), samePointer // 1 次构造，所有调用者同一实例
}
