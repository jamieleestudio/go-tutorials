package concurrent

import (
	"context"
	"time"
)

// CancelDemo context.WithCancel：主 goroutine 主动通知 worker 停止。
//
// 确定性模式要点：
//   - worker 不裸 sleep 竞速，而是把取消检查做进每个工作单元的 select 里 ——
//     取消能【立刻打断】进行中的等待；
//   - 时间线是确定的：t=0 第 1 个 tick（20ms）完成；t=30ms 主 goroutine cancel；
//     t=20ms worker 开始第 2 个 tick，select 上 Done(30ms) 先于计时器(40ms) → 取消命中；
//   - 结果恒为：canceled=true，ticks=1。
func CancelDemo() (canceled bool, ticks int) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // 惯例：无论走哪条路径都 cancel，释放 ctx 资源（go vet 会查）

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 5 { // 计划 5 个 tick，每个"工作单元"耗时 20ms
			select {
			case <-ctx.Done(): // 收到取消信号：立即退出，不再干活
				canceled = true
				return
			case <-time.After(20 * time.Millisecond): // 模拟一段工作
				ticks++
			}
		}
	}()

	time.Sleep(30 * time.Millisecond) // 主 goroutine：约 1.5 个 tick 后决定取消
	cancel()
	<-done // 等 worker 确认退出
	return
}

// TimeoutDemo context.WithTimeout：给整段工作一个截止时间。
// 模拟的工作需要 50ms：给 200ms → 正常完成（false）；给 5ms → 超时放弃（true）。
// 对照 Java：future.get(timeout) / CompletableFuture.orTimeout。
func TimeoutDemo(timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	select {
	case <-time.After(50 * time.Millisecond): // 模拟 50ms 的工作
		return false // 干完了，没超时
	case <-ctx.Done():
		return true // 截止时间到了
	}
}

// valueKey 私有键类型：WithValue 的 key 必须用【未导出的自定义类型】——
// 保证别的包不可能撞键（对应 Java ThreadLocal 的"线程私有"思路，
// 但 Go 的作用域是"一次请求的调用链"而不是线程）。
type valueKey struct{}

// userKey 演示第二个键。
type userKey struct{}

// ValueDemo context.WithValue：跨 API 边界携带请求级数据（traceID、登录用户等）。
// 每次WithValue 返回派生的新 ctx，旧的不可变 —— 天然并发安全。
// 官方告诫：只放请求作用域的数据，别拿它当普通函数参数用。
func ValueDemo() (user, request string) {
	ctx := context.WithValue(context.Background(), valueKey{}, "trace-2026")
	ctx = context.WithValue(ctx, userKey{}, "小明")
	request, _ = ctx.Value(valueKey{}).(string) // 取值 + 类型断言，两步一体
	user, _ = ctx.Value(userKey{}).(string)
	return
}
