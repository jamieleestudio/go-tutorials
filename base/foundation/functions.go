package foundation

import "fmt"

// Divide 多返回值 —— Go 错误处理的基石：(结果, error)。
// 对照 Java：只能返回一个值，异常走 throw；Go 把"失败"变成普通的返回值。
func Divide(a, b int) (int, error) {
	if b == 0 {
		return 0, fmt.Errorf("除数不能为 0: %d/%d", a, b)
	}
	return a / b, nil
}

// NamedReturn 命名返回值：自带文档化含义；配合 defer 还能修改返回值（见 errors.go）。
// 顺带演示 Go 的多赋值交换（对应 Java 的 temp 三行）。
func NamedReturn(min, max int) (lo, hi int) {
	if min > max {
		min, max = max, min
	}
	lo, hi = min, max
	return // 裸 return（小函数里可读；长函数建议显式 return）
}

// Sum 可变参数 —— 对应 Java 的 int... nums；调用方可用 nums... 展开 slice。
func Sum(nums ...int) int {
	total := 0
	for _, n := range nums {
		total += n
	}
	return total
}

// ClosureCounter 闭包：函数值捕获外部变量并保持状态。
// 对照 Java：lambda 只能捕获 effectively final 的变量；Go 没有这个限制。
func ClosureCounter() func() int {
	count := 0
	return func() int {
		count++
		return count
	}
}

// Apply 函数是一等公民，可以当参数传 —— 对应 Java 的函数式接口 Function<Integer,Integer>。
func Apply(nums []int, f func(int) int) []int {
	out := make([]int, len(nums))
	for i, n := range nums {
		out[i] = f(n)
	}
	return out
}

// DeferOrder defer 后进先出；defer 的闭包可以修改【命名返回值】—— 对应 Java 的 finally。
func DeferOrder() (log []string) {
	defer func() { log = append(log, "A(后定义先执行)") }()
	defer func() { log = append(log, "B(后定义先执行)") }()
	log = append(log, "body")
	return
}

// DeferCapture defer 的两个易错点：
//   - defer 语句的【参数】在 defer 执行到时立即求值；
//   - 【闭包】捕获的是变量引用，取最终值。
func DeferCapture() (log []int) {
	x := 1
	defer func(v int) { log = append(log, v) }(x) // 立即求值：捕获 x=1
	x = 2
	defer func() { log = append(log, x) }() // 闭包引用：最终 x=2；且后定义先执行
	log = append(log, 0)
	return
}
