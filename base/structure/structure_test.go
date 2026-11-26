package structure

import (
	"encoding/json"
	"testing"
)

func TestArrayIsValue(t *testing.T) {
	a, b := ArrayIsValue()
	if a[0] != 1 || b[0] != 99 {
		t.Fatalf("array copy wrong: %v %v", a, b)
	}
}

func TestSliceBasicsAndTraps(t *testing.T) {
	s, l, c := SliceBasics()
	if l != 4 || c != 5 || s[3] != 4 {
		t.Fatalf("slice basics: %v %d %d", s, l, c)
	}

	// 共享底层数组的写穿
	s1 := SliceAliasing()
	want := []int{1, 99, 3, 88, 5}
	for i := range want {
		if s1[i] != want[i] {
			t.Fatalf("aliasing wrong: %v", s1)
		}
	}

	// 三索引切片杜绝写穿
	s2 := SliceFullExpr()
	want2 := []int{1, 2, 3, 4, 5}
	for i := range want2 {
		if s2[i] != want2[i] {
			t.Fatalf("full expr wrong: %v", s2)
		}
	}

	// copy 解耦
	src, dst := CopySlice()
	if src[0] != 1 || dst[0] != 99 {
		t.Fatalf("copy wrong: %v %v", src, dst)
	}

	// 删除中间元素
	got := DeleteElem([]int{1, 2, 3, 4}, 1)
	if got[0] != 1 || got[1] != 3 || got[2] != 4 || len(got) != 3 {
		t.Fatalf("delete wrong: %v", got)
	}

	// nil 切片可 append
	okNil, okEmpty := NilVsEmpty()
	if !okNil || !okEmpty {
		t.Fatal("nil/empty slice append wrong")
	}
}

func TestMapSemantics(t *testing.T) {
	found, okFound, missing, okMissing := MapBasics()
	if found != 1 || !okFound || missing != 0 || okMissing {
		t.Fatalf("comma-ok wrong: %d %v %d %v", found, okFound, missing, okMissing)
	}

	counts := Counting([]string{"go", "go", "rust"})
	if counts["go"] != 2 || counts["rust"] != 1 {
		t.Fatalf("counting wrong: %v", counts)
	}

	viaM1, viaM2 := MapIsReference()
	if viaM1 != 99 || viaM2 != 99 {
		t.Fatalf("map reference wrong: %d %d", viaM1, viaM2)
	}

	year := AnyValue()["year"].(int) // any 里的值要断言回来
	if year != 2009 {
		t.Fatalf("any value wrong: %d", year)
	}
}

func TestReceiverKinds(t *testing.T) {
	valueX, pointerX := Move()
	if valueX != 1 || pointerX != 11 {
		t.Fatalf("receivers wrong: %d %d", valueX, pointerX)
	}
}

func TestCompositionOverInheritance(t *testing.T) {
	d := NewDog("旺财", "柴犬")
	if got := d.Describe(); got != "我是 旺财，是只 柴犬" {
		t.Fatalf("shadowing method wrong: %q", got)
	}
	if got := d.Animal.Describe(); got != "我是 旺财" {
		t.Fatalf("explicit parent call wrong: %q", got)
	}
}

func TestStructTagJSON(t *testing.T) {
	raw, err := json.Marshal(UserDTO{Name: "小明"})
	if err != nil {
		t.Fatal(err)
	}
	// omitempty：Age 零值被省略 —— 对应 Jackson 的 @JsonInclude(NON_NULL)
	if string(raw) != `{"name":"小明"}` {
		t.Fatalf("json wrong: %s", raw)
	}
	var back UserDTO
	if err := json.Unmarshal([]byte(`{"name":"a","age":3}`), &back); err != nil {
		t.Fatal(err)
	}
	if back.Name != "a" || back.Age != 3 {
		t.Fatalf("unmarshal wrong: %+v", back)
	}
}

func TestImplicitInterfaces(t *testing.T) {
	greets := GreetAll([]Speaker{Robot{Name: "R2"}, Human{Name: "小明"}})
	if greets[0] != "哔哔：R2" || greets[1] != "你好，我是 小明" {
		t.Fatalf("polymorphism wrong: %v", greets)
	}
	both := DescribeAll([]SpeakerNamer{Robot{Name: "R2"}})
	if both[0] != "robot-R2 → 哔哔：R2" {
		t.Fatalf("composition interface wrong: %v", both)
	}
}

func TestTypeAssertionAndSwitch(t *testing.T) {
	isString, str, isInt, i := TypeAssert("hi")
	if !isString || str != "hi" || isInt || i != 0 {
		t.Fatalf("assert string wrong")
	}
	isString, str, isInt, i = TypeAssert(7)
	if isString || isInt == false || i != 7 || str != "" {
		t.Fatalf("assert int wrong")
	}
	if got := TypeSwitch([]int{1, 2}); got != "整数切片:[1 2]" {
		t.Fatalf("type switch wrong: %q", got)
	}
	if got := TypeSwitch(true); got != "未知类型" {
		t.Fatalf("type switch default wrong: %q", got)
	}
}

func TestGenerics(t *testing.T) {
	if got := Sum([]int{1, 2, 3}); got != 6 {
		t.Fatalf("sum int wrong: %d", got)
	}
	if got := Sum([]float64{1.5, 2.5}); got != 4.0 {
		t.Fatalf("sum float wrong: %v", got)
	}

	nums := []int{1, 2, 3, 4, 5, 6}
	doubled := Map(nums, func(n int) int { return n * 2 })
	evens := Filter(nums, func(n int) bool { return n%2 == 0 })
	total := Reduce(nums, 0, func(acc, n int) int { return acc + n })
	if doubled[1] != 4 || len(evens) != 3 || total != 21 {
		t.Fatalf("stream-like wrong: %v %v %d", doubled, evens, total)
	}

	var s Stack[string]
	s.Push("a")
	s.Push("b")
	if v, ok := s.Pop(); !ok || v != "b" {
		t.Fatalf("generic stack wrong")
	}
	if _, ok := s.Pop(); !ok {
		t.Fatal("second pop should succeed")
	}
	if _, ok := s.Pop(); ok {
		t.Fatal("empty pop should return false")
	}
}

func TestNestedCorrespondence(t *testing.T) {
	if got := LocalType(); got != "(1,2)" {
		t.Fatalf("local type wrong: %q", got)
	}
	if got := MakeTask("Go").Run(); got != "hi, Go" {
		t.Fatalf("closure adapter wrong: %q", got)
	}
	outer, inner := Shadowing()
	if outer != 1 || inner != 2 {
		t.Fatalf("shadowing wrong: %d %d", outer, inner)
	}
}
