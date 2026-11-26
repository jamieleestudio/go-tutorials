// Package foundation 语法基础第一课：变量、类型、常量与枚举。
//
// 对应 java-tutorials 的 base/se/foundation（EnumTest 等）。
// 主线：给 Java 开发者看 Go 的同名概念长什么样，差异点逐条标注。
package foundation

import (
	"strconv"
	"unicode/utf8"
)

// ==== 变量声明：var 与 := ====

// DeclareVars 三种声明方式：var 显式类型 / := 短声明（只能在函数内）/ var + 显式初始化。
func DeclareVars() (int, string, bool) {
	var a int    // var 声明：零值初始化
	b := "hello" // 短声明：类型推断，最常用
	var c bool = true
	return a + len(b), b + "!", c // len 按字节计数；字符串拼接用 +
}

// Sample 零值演示用结构体。
type Sample struct {
	Num    int
	Str    string
	Flag   bool
	Items  []int
	Lookup map[string]int
	Ptr    *int
}

// ZeroValues Go 没有"未初始化"—— 每个类型都有确定的零值：
// 数字 0、字符串 ""、bool false、slice/map/指针 nil。
// 对照 Java：引用类型默认 null；Go 的 nil 切片 len==0 且 append 可用。
func ZeroValues() Sample {
	var s Sample // 不写任何初始化
	return s
}

// ExplicitConversion Go 的类型转换必须显式 —— Java 的隐式拓宽（int→long→double）不存在。
// 教学点：不写转换直接混合类型运算会编译错误，这是 Go 防"静默精度丢失"的设计。
func ExplicitConversion() (float64, int64, string) {
	i := 42
	f := float64(i) / 7 // 显式转换后再除；不转换的话 42/7 是整数除法得 6（这里恰好相等，但对 7/2 就不同了）
	return f, int64(i), strconv.Itoa(i)
}

// StringIsBytes Go 字符串是 UTF-8 字节序列：
//   - len 按字节计数；
//   - range 按 rune（Unicode 码点）迭代，自动解码。
//
// 对照 Java：String 以 UTF-16 存储，charAt 取的是 UTF-16 代码单元；
// Go 的 rune 才对应"一个字符"。
func StringIsBytes() (bytes int, runes int, firstRune rune) {
	s := "你好Go"
	bytes = len(s)                    // 3+3+1+1 = 8 字节
	runes = utf8.RuneCountInString(s) // 4 个字符
	for _, r := range s {             // range 解码出 rune
		firstRune = r // '你'
		break
	}
	return
}

// Weekday 枚举：Go 没有 enum 关键字，惯例是"命名类型 + iota 常量块"。
// 对应 Java 的 enum Weekday { SUNDAY, MONDAY, ... }。
type Weekday int

const (
	Sunday  Weekday = iota // 0，后续自动递增
	Monday                 // 1
	Tuesday                // 2
	Wednesday
	Thursday
	Friday
	Saturday // 6
)

// String 实现 fmt.Stringer —— 对应 Java enum 的 name()/toString()。
func (w Weekday) String() string {
	names := [...]string{"星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六"}
	if w < Sunday || w > Saturday {
		return "未知"
	}
	return names[w]
}

// iota 表达式：常量块里每一条都复用上一条的表达式，iota 按行递增。
// 常见用法是位标志（对应 Java 的 EnumSet 位优化或手写常量）。
const (
	Read  = 1 << iota // 1<<0 = 1
	Write             // 1<<1 = 2
	Exec              // 1<<2 = 4
)

// Builtins Go 1.21 内置 min/max（对应 Java Math.min/max）。
func Builtins() (lo, hi int) {
	return min(3, 7), max(3, 7)
}
