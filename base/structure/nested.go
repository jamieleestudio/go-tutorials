package structure

import "fmt"

// Java 的四种嵌套类在 Go 里的对应物：
//
//	静态嵌套类 → 包级 struct（Go 本来就没有"外部类"的概念）
//	内部类     → struct 嵌入（组合）
//	局部类     → 函数内定义类型（Go 支持函数内 type，但不能再挂方法）
//	匿名类     → 函数值/闭包 + 函数类型适配器

// LocalType 局部类：函数内定义、函数外不可见（对应 Java local class）。
// 注意：函数内类型不能再定义方法 —— 需要方法就提升到包级。
func LocalType() string {
	type point struct{ x, y int } // 函数内类型
	p := point{1, 2}
	return fmt.Sprintf("(%d,%d)", p.x, p.y)
}

// Task 接口。
type Task interface {
	Run() string
}

// TaskFunc 函数类型适配器：给"函数"补上接口方法。
// 这是 Java lambda 直接实现函数式接口的 Go 对应物 —— 函数本身没有方法，
// 用一个一层薄适配器把 func 变成接口实现。
type TaskFunc func() string

func (f TaskFunc) Run() string { return f() }

// MakeTask 匿名类的 Go 对应物：返回一个闭包适配成的接口实现。
// 对照 Java：return () -> "hi, " + name;
func MakeTask(name string) Task {
	return TaskFunc(func() string { return "hi, " + name })
}

// Shadowing 变量遮蔽（对应 Java 的 ShadowTest）——
// 内层块的 := 会新建一个同名变量，遮蔽外层；这是 Go 常见 bug 源。
// 建议：开启 go vet / shadow 检查。
func Shadowing() (outer, inner int) {
	n := 1
	if true {
		n := 2 // 遮蔽：这是另一个变量
		inner = n
	}
	outer = n
	return
}
