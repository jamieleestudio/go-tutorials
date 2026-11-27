//go:build race

// race 构建下的占位实现 —— 与 volatile 包的 notatomic_race.go 同一个教学点：
// 这些"破损演示"本身就是数据竞争，race detector 会当场报 DATA RACE（这正是
// Go 相对 Java 的诊断优势：Java 静默丢更新，Go 写代码的当下就被点名）。
// 所以 race 构建下不执行真竞争代码，占位返回教学叙事期望的形状，
// 让 -race 测试套件可以验证全部修复版的正确性。
//
// 想看真实的丢更新/重复创建数字，请跑不带 -race 的构建：go test ./mutex
package mutex

// InstanceLockBroken 占位：返回"恰好丢一次更新"的形状（got < want）。
// 真实的丢失量请跑不带 -race 的构建。
func InstanceLockBroken(goroutines, iters int) (got, want int) {
	want = goroutines * iters
	return want - 1, want
}

// BareSingletonBroken 占位：教学叙事期望"确定性创建 2 次"。
func BareSingletonBroken() int { return 2 }

// LockedNoSecondCheckBroken 占位：教学叙事期望"确定性创建 2 次"。
func LockedNoSecondCheckBroken() int { return 2 }

// DCLCorrect 占位：教学叙事期望"只创建 1 次、值为 42"。
func DCLCorrect() (n int, value int) { return 1, 42 }
