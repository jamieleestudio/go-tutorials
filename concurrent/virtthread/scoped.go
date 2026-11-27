package virtthread

import (
	"context"
	"fmt"
)

// ScopedValue 的 Go 答案 —— 对应 Java ScopedValueDemo（JEP 446）。
//
// Java ScopedValue 相对 ThreadLocal 的三大优势（原注释摘录）：
//   1. 单向传递/不可变：where(USER, "Alice").run(...) 绑定后闭包内只读；
//   2. 无泄漏：run 结束绑定自动失效，无需手动 remove()；
//   3. 海量虚拟线程下开销远低于 ThreadLocal。
//
// Go 没有这层魔法 —— goroutine 从设计上就没有"隐式执行上下文"，一切靠显式传参。
// context.Context 是官方答案，对照三大优势逐条看：
//
//   1. 不可变：WithValue 返回新 ctx，旧 ctx 不受影响（单向）✓
//   2. 无泄漏：值随 ctx 生命周期，取消后可 GC（但【没有】作用域自动消失 —— 忘记传
//      ctx 或塞错值就是事故，Go 社区的纪律是"key 必须是非导出类型"）△
//   3. 开销：每层 WithValue 包装一个节点，深层链有 O(depth) 读成本 —— Java
//      ScopedValue 是数组快照。Go 官方也警告"context 值不要当参数用"，能显式
//      传参就显式传参。△
//
// 教学结论：Java ScopedValue 解决"免传参读上下文"，Go 用"显式传 ctx + 纪律"达成
// 同样的工程目标 —— 又一次"关键字 vs 结构"的对照。

// userKey 非导出类型 —— context.WithValue 的官方要求（防跨包键冲突）。
type userKey struct{}

// RequestScoped 对应 ScopedValueDemo 的 where(USER, "Alice").run(...)：
// 构造携带用户身份的 ctx，深入调用链免传参读取。
func RequestScoped(parent context.Context, user string) context.Context {
	return context.WithValue(parent, userKey{}, user)
}

// HandleRequest 对应 handleRequest：深层调用链直接从 ctx 读用户（对应 USER_CONTEXT.get()），
// 再继续传给 saveToDatabase —— 演示"读上下文 + 向下传播"两个动作。
func HandleRequest(ctx context.Context, method string) string {
	user, _ := ctx.Value(userKey{}).(string) // 对应 USER_CONTEXT.get()
	audit := SaveToDatabase(ctx)             // 向下传播（对应 ScopedValue 在闭包内继续可见）
	return fmt.Sprintf("%s | 当前用户: %s | 审计操作人: %s", method, user, audit)
}

// SaveToDatabase 更深一层：同样从 ctx 读（对应 saveToDatabase 里的第二次 get）。
func SaveToDatabase(ctx context.Context) string {
	user, _ := ctx.Value(userKey{}).(string)
	return user
}

// UnboundOutsideScope 对应 ScopedValueDemo 末尾：主线程 USER_CONTEXT.isBound() == false。
// ctx 不带值 → 读到 nil —— "作用域外取不到"在 Go 里是显式的（nil 断言，不是异常）。
func UnboundOutsideScope() bool {
	ctx := context.Background() // 不经过 WithValue
	user, ok := ctx.Value(userKey{}).(string)
	return user == "" && !ok
}
