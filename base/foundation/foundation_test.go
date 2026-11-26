package foundation

import (
	"errors"
	"slices"
	"testing"
)

func TestDeclareVars(t *testing.T) {
	a, b, c := DeclareVars()
	if a != 5 || b != "hello!" || !c {
		t.Fatalf("got %d %q %v", a, b, c)
	}
}

func TestZeroValues(t *testing.T) {
	s := ZeroValues()
	if s.Num != 0 || s.Str != "" || s.Flag != false {
		t.Fatalf("basic zero values wrong: %+v", s)
	}
	if s.Items != nil || s.Lookup != nil || s.Ptr != nil {
		t.Fatalf("nil zero values wrong: %+v", s)
	}
	if len(s.Items) != 0 {
		t.Fatalf("nil slice len should be 0")
	}
}

func TestExplicitConversion(t *testing.T) {
	f, big, s := ExplicitConversion()
	if f != 6.0 || big != 42 || s != "42" {
		t.Fatalf("got %v %v %q", f, big, s)
	}
}

func TestStringIsBytes(t *testing.T) {
	bytes, runes, first := StringIsBytes()
	if bytes != 8 || runes != 4 || first != '你' {
		t.Fatalf("got bytes=%d runes=%d first=%q", bytes, runes, first)
	}
}

func TestWeekdayEnum(t *testing.T) {
	if Sunday.String() != "星期日" || Friday.String() != "星期五" {
		t.Fatalf("enum string wrong")
	}
	if Weekday(99).String() != "未知" {
		t.Fatalf("out-of-range should be 未知")
	}
	// 对照 Java：enum 有类型安全；Go 的 iota 枚举本质是 int，越界要自己防御（如上）。
	if Read != 1 || Write != 2 || Exec != 4 {
		t.Fatalf("iota expression wrong: %d %d %d", Read, Write, Exec)
	}
}

func TestBuiltins(t *testing.T) {
	if lo, hi := Builtins(); lo != 3 || hi != 7 {
		t.Fatalf("min/max wrong: %d %d", lo, hi)
	}
}

func TestIfWithInit(t *testing.T) {
	if got := IfWithInit("hello"); got != "长字符串(5)" {
		t.Fatalf("got %q", got)
	}
	if got := IfWithInit("ab"); got != "短字符串" {
		t.Fatalf("got %q", got)
	}
	if got := IfWithInit(""); got != "空字符串" {
		t.Fatalf("got %q", got)
	}
}

func TestLoops(t *testing.T) {
	a, b, c, d := Loops()
	if a != 55 || b != 55 || c != 55 || d != 10 {
		t.Fatalf("got %d %d %d %d", a, b, c, d)
	}
	if got := RangeOverInt(); got != 10 {
		t.Fatalf("range over int: got %d", got)
	}
}

func TestSwitchForms(t *testing.T) {
	if SwitchBasics(Sunday) != "周末" || SwitchBasics(Wednesday) != "工作日" {
		t.Fatalf("switch basics wrong")
	}
	if SwitchWithInit(4) != "余1" {
		t.Fatalf("switch with init wrong")
	}
	if SwitchNoExpr(95) != "优秀" || SwitchNoExpr(60) != "及格" || SwitchNoExpr(10) != "不及格" {
		t.Fatalf("no-expr switch wrong")
	}
}

func TestLabeledBreak(t *testing.T) {
	if got := LabeledBreak(); got != 6 {
		t.Fatalf("got %d, want 6", got)
	}
}

func TestDivideAndMultiReturn(t *testing.T) {
	q, err := Divide(10, 2)
	if err != nil || q != 5 {
		t.Fatalf("divide: %d %v", q, err)
	}
	if _, err := Divide(1, 0); err == nil {
		t.Fatal("expected error for zero divisor")
	}
	if lo, hi := NamedReturn(9, 3); lo != 3 || hi != 9 {
		t.Fatalf("named return: %d %d", lo, hi)
	}
}

func TestVariadicAndSpread(t *testing.T) {
	if Sum(1, 2, 3) != 6 {
		t.Fatal("variadic wrong")
	}
	nums := []int{4, 5, 6}
	if Sum(nums...) != 15 { // slice 展开传给可变参数
		t.Fatal("spread wrong")
	}
}

func TestClosureAndFirstClassFunc(t *testing.T) {
	c1 := ClosureCounter()
	if c1() != 1 || c1() != 2 {
		t.Fatal("closure state wrong")
	}
	c2 := ClosureCounter() // 每次调用创建独立的捕获环境
	if c2() != 1 {
		t.Fatal("independent closure wrong")
	}
	got := Apply([]int{1, 2, 3}, func(n int) int { return n * 2 })
	if got[0] != 2 || got[2] != 6 {
		t.Fatalf("apply wrong: %v", got)
	}
}

func TestDeferSemantics(t *testing.T) {
	order := DeferOrder()
	want := []string{"body", "B(后定义先执行)", "A(后定义先执行)"}
	if len(order) != 3 || order[0] != want[0] || order[1] != want[1] || order[2] != want[2] {
		t.Fatalf("defer LIFO wrong: %v", order)
	}
	log := DeferCapture()
	want2 := []int{0, 2, 1} // body → 闭包 defer（最终值2）→ 参数 defer（立即求值1）
	if !slices.Equal(log, want2) {
		t.Fatalf("defer capture wrong: %v", log)
	}
}

func TestErrorPatterns(t *testing.T) {
	isValidation, reason := InspectLoadError("")
	if !isValidation || reason != "不能为空" {
		t.Fatalf("errors.As wrong: %v %q", isValidation, reason)
	}
	if isValidation, _ := InspectLoadError("ok"); isValidation {
		t.Fatal("no error expected")
	}
	if !CheckNotFound("ghost") || !CheckNotFound("") {
		t.Fatal("errors.Is should find wrapped sentinel")
	}
	if CheckNotFound("real-user") {
		t.Fatal("unexpected sentinel")
	}
	if q, err := SafeDivide(6, 2); err != nil || q != 3 {
		t.Fatalf("safe divide: %d %v", q, err)
	}
	_, err := SafeDivide(1, 0)
	if err == nil || !errors.Is(err, ErrDivideByZero) {
		t.Fatalf("panic/recover 包装链断了: %v", err)
	}
}
