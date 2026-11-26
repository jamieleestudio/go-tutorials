package concurrent

import (
	"sync"
	"sync/atomic"
)

// MutexDemo 10000 个 goroutine 抢同一把锁自增 —— Java ReentrantLockTest 的直接对应物。
// 每次自增都在 mu 的临界区内，最终值精确。
func MutexDemo(goroutines int) int {
	var (
		mu sync.Mutex
		n  int
		wg sync.WaitGroup
	)
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			n++
			mu.Unlock()
		}()
	}
	wg.Wait()
	return n
}

// CheckAndRemove ReentrantLockTest 的核心语义："检查 map 里有没有 '1'，有就删"。
//
// 关键点：检查 + 删除必须是【同一个临界区】里的原子操作 ——
// 拆成两段锁（或干脆不加锁）就是经典 check-then-act 竞态：
// 多个 goroutine 会同时看到 "1" 存在，然后都去删。
// 对照 Java：reentrantLock.lock(); try { ... } finally { reentrantLock.unlock(); }
func CheckAndRemove(goroutines int) (removed int) {
	var (
		mu  sync.Mutex
		set = map[string]struct{}{"1": {}}
		wg  sync.WaitGroup
	)
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			defer mu.Unlock() // 对应 Java 的 finally unlock
			if _, ok := set["1"]; ok {
				delete(set, "1")
				removed++ // 10000 个 goroutine 里只可能有 1 个成功
			}
		}()
	}
	wg.Wait()
	return
}

// TryLockDemo sync.Mutex.TryLock（Go 1.18+）：拿不到锁立即返回 false，绝不阻塞 ——
// 对应 Java 的 tryLock()。典型场景：定时任务防重入、"忙则跳过"。
func TryLockDemo() (held, acquired bool) {
	var mu sync.Mutex
	mu.Lock()
	held = mu.TryLock() // false：锁已被持有，立即失败
	mu.Unlock()
	acquired = mu.TryLock() // true：已释放，这次能拿到
	mu.Unlock()
	return
}

// RWMutexDemo 读写锁：读锁共享（多个读者可同时进入）、写锁独占 ——
// 对照 Java 的 ReentrantReadWriteLock.readLock()/writeLock()。
//
// 教学点：3 个读者【同时】持有 RLock 才算证明"共享"——
// 如果换成普通 Mutex，第二个 RLock 会死锁在这里。
func RWMutexDemo() (value int, sharedReads int) {
	var (
		mu       sync.RWMutex
		m        = map[string]int{"k": 0}
		acquired = make(chan struct{}, 3)
		release  = make(chan struct{})
		wg       sync.WaitGroup
	)
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.RLock()
			acquired <- struct{}{} // 我已进入读锁
			<-release              // 等"三个读者都已进入"的信号
			mu.RUnlock()
		}()
	}
	for range 3 {
		<-acquired // 三个读者此刻同时持锁 —— 共享性得证
		sharedReads++
	}
	close(release)
	wg.Wait()

	mu.Lock() // 写锁独占
	m["k"] = 42
	mu.Unlock()

	value = m["k"] // 写者已结束且是单线程读，无需再加锁
	return
}

// OnceDemo sync.Once：无论多少 goroutine 调用 Do，f 只执行一次 ——
// 对照 Java 单例的双检锁 / Holder 惯用法，Go 一行 sync.Once 搞定。
func OnceDemo(goroutines int) int32 {
	var (
		once  sync.Once
		calls atomic.Int32
		wg    sync.WaitGroup
	)
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			once.Do(func() {
				calls.Add(1) // 只有一个 goroutine 能进这里
			})
		}()
	}
	wg.Wait()
	return calls.Load()
}
