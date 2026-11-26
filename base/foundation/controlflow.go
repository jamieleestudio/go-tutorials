package foundation

import "strconv"

// IfWithInit if 可以带初始化语句，变量作用域限定在 if/else 内。
// 对照 Java：只能先写 int n = s.length(); if (n > 3)，n 会泄漏到外层。
func IfWithInit(s string) string {
	if n := len(s); n > 3 {
		return "长字符串(" + strconv.Itoa(n) + ")"
	} else if n > 0 {
		return "短字符串"
	} else {
		return "空字符串"
	}
}

// Loops Go 只有 for 一种循环（没有 while / do-while），四种形态全覆盖。
func Loops() (classic, whileStyle, infinite, rangeSum int) {
	for i := 1; i <= 10; i++ { // 经典三段式
		classic += i
	}
	i := 1
	for i <= 10 { // 当 while 用：只有条件
		whileStyle += i
		i++
	}
	k := 1
	for { // 无限循环，靠 break 退出
		if k > 10 {
			break
		}
		infinite += k
		k++
	}
	for _, v := range []int{1, 2, 3, 4} { // range 迭代切片
		rangeSum += v
	}
	return
}

// RangeOverInt Go 1.22 起 range 可以直接迭代整数（for i := range 5 → i: 0..4）。
func RangeOverInt() int {
	total := 0
	for i := range 5 {
		total += i
	}
	return total
}

// SwitchBasics switch 自动 break，不会像 Java 那样穿透（显式写 fallthrough 才会）。
// case 可以带多个值 —— 对应 Java 的 case SUNDAY: case SATURDAY: 连写。
func SwitchBasics(day Weekday) string {
	switch day {
	case Saturday, Sunday:
		return "周末"
	case Monday, Tuesday, Wednesday, Thursday, Friday:
		return "工作日"
	default:
		return "未知"
	}
}

// SwitchWithInit switch 同样支持初始化语句。
func SwitchWithInit(n int) string {
	switch m := n % 3; m {
	case 0:
		return "余0"
	case 1:
		return "余1"
	default:
		return "余2"
	}
}

// SwitchNoExpr 无表达式 switch —— 对应 Java 的 if-else-if 长链，可读性更好。
func SwitchNoExpr(score int) string {
	switch {
	case score >= 90:
		return "优秀"
	case score >= 60:
		return "及格"
	default:
		return "不及格"
	}
}

// LabeledBreak 带 label 的 break/continue（对应 Java 的 outer: 标签循环）。
func LabeledBreak() (pairs int) {
outer:
	for i := 1; i <= 3; i++ {
		for j := 1; j <= 3; j++ {
			if j > i {
				continue outer // 直接继续外层下一轮
			}
			pairs++
		}
	}
	return
}
