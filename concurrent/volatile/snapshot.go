package volatile

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Config 不可变快照 —— 对应 Java VolatileReferencePublicationDemo 的 final 字段 Config。
// 惯例：结构体一旦发布就不改字段，每次变更 new 一个新的整体替换引用。
type Config struct {
	Name    string
	Version int
}

// validPair 读方的一致性校验：名字与版本必须配对（v1:1 / v2:2），撕裂组合 = 事故。
func (c *Config) validPair() bool {
	return (c.Name == "v1" && c.Version == 1) || (c.Name == "v2" && c.Version == 2)
}

// SnapshotPublish 对应 VolatileReferencePublicationDemo：volatile 引用 + 整体替换。
//
// Java 版：volatile Config config，4 个读线程忙读 1 秒，永远读不到"v1 名字 + version 2"
// 的撕裂组合。volatile 引用写（release）保证：读线程要么看到旧对象要么看到新对象，
// 且对象的构造结果全部可见 —— final 字段 + volatile 引用 = 安全发布。
//
// Go 版答案：atomic.Pointer[Config]。语义与 Java 版逐条对应：
//   - Store = volatile 写（release：之前的构造写全部可见）
//   - Load  = volatile 读（acquire）
//   - Go 无 final，但"发布后不改字段"是纪律；真正不可变要靠不提供写方法 + 文档
func SnapshotPublish(readWindow time.Duration) (observed int64, torn bool, final Config) {
	var ref atomic.Pointer[Config]
	ref.Store(&Config{Name: "v1", Version: 1})

	var (
		observedCount atomic.Int64
		tornFlag      atomic.Bool
		stop          = make(chan struct{})
		wg            sync.WaitGroup
	)
	// 4 个读线程忙读（对应 Java 的 4 reader）
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				c := ref.Load() // 原子读快照指针
				if c == nil {
					tornFlag.Store(true) // 不可能发生：初始已 Store
					continue
				}
				if !c.validPair() {
					tornFlag.Store(true) // 撕裂组合：v1 名字配 v2 版本号
					continue
				}
				observedCount.Add(1)
			}
		}()
	}

	time.Sleep(50 * time.Millisecond)
	ref.Store(&Config{Name: "v2", Version: 2}) // 整体替换引用（对应 config = new Config("v2",2)）
	time.Sleep(readWindow)
	close(stop)
	wg.Wait()

	last := ref.Load()
	return observedCount.Load(), tornFlag.Load(), *last
}

// String 组装演示输出（对应 Java 版的 reader observed N snapshots 行）。
func SnapshotString() string {
	observed, torn, final := SnapshotPublish(80 * time.Millisecond)
	return fmt.Sprintf("observed=%d torn=%v final=%s:%d", observed, torn, final.Name, final.Version)
}
