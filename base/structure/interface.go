package structure

import "fmt"

// Speaker 接口：只声明行为契约。
type Speaker interface {
	Speak() string
}

// Namer 另一个小接口。
type Namer interface {
	Label() string
}

// SpeakerNamer 接口组合（对应 Java 接口 extends 接口）。
type SpeakerNamer interface {
	Speaker
	Namer
}

// Robot 隐式实现 Speaker + Namer —— 不写 implements，编译器自动判定方法集。
// 这是 Go 与 Java 最大的差异之一：实现方与接口定义方完全解耦，
// 你甚至可以给"别人的类型"补接口（在同包里写方法）。
type Robot struct{ Name string }

func (r Robot) Speak() string { return "哔哔：" + r.Name }
func (r Robot) Label() string { return "robot-" + r.Name }

// Human 同样隐式实现。
type Human struct{ Name string }

func (h Human) Speak() string { return "你好，我是 " + h.Name }
func (h Human) Label() string { return "human-" + h.Name }

// GreetAll 多态：接口切片统一调用（对应 List<Speaker>）。
func GreetAll(speakers []Speaker) []string {
	out := make([]string, 0, len(speakers))
	for _, s := range speakers {
		out = append(out, s.Speak())
	}
	return out
}

// DescribeAll 组合接口的使用（每个元素既有 Speak 又有 Label）。
func DescribeAll(ns []SpeakerNamer) []string {
	out := make([]string, 0, len(ns))
	for _, n := range ns {
		out = append(out, n.Label()+" → "+n.Speak())
	}
	return out
}

// TypeAssert 类型断言 comma-ok —— 对应 Java 的 instanceof + 强转，但一步完成。
func TypeAssert(v any) (isString bool, str string, isInt bool, i int) {
	if s, ok := v.(string); ok {
		isString, str = ok, s
	}
	if n, ok := v.(int); ok {
		isInt, i = ok, n
	}
	return
}

// TypeSwitch 类型 switch（对应 Java 的连续 instanceof 分支）。
func TypeSwitch(v any) string {
	switch x := v.(type) {
	case string:
		return "字符串:" + x
	case int:
		return "整数:" + fmt.Sprint(x)
	case []int:
		return fmt.Sprintf("整数切片:%v", x)
	default:
		return "未知类型"
	}
}
