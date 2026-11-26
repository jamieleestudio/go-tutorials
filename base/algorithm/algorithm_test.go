package algorithm

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

func TestQuickSortThreePartitions(t *testing.T) {
	// 与 Java QuickSort.main 完全一致的输入
	base := []int{8, 1, 3, 6, 3, 2, 9}
	want := []int{1, 2, 3, 3, 6, 8, 9}

	strategies := []struct {
		name      string
		partition func([]int, int, int) int
	}{
		{"DoublePointerSwap", DoublePointerSwap},
		{"DoublePointerHole", DoublePointerHole},
		{"SinglePointer", SinglePointer},
	}
	for _, s := range strategies {
		arr := slices.Clone(base)
		QuickSortBy(arr, 0, len(arr)-1, s.partition)
		if !slices.Equal(arr, want) {
			t.Fatalf("%s: %v, want %v", s.name, arr, want)
		}
	}

	// 默认入口（双边交换法）
	arr := slices.Clone(base)
	QuickSort(arr, 0, len(arr)-1)
	if !slices.Equal(arr, want) {
		t.Fatalf("QuickSort default: %v, want %v", arr, want)
	}
}

func TestQuickSortRandomVsStdlib(t *testing.T) {
	// 固定种子 200 个随机数，与标准库排序对照 —— 证明三种 partition 都"真的对"
	rng := rand.New(rand.NewPCG(2026, 2026))
	arr := make([]int, 200)
	for i := range arr {
		arr[i] = rng.IntN(1000)
	}
	want := slices.Clone(arr)
	slices.Sort(want)

	for _, s := range []struct {
		name      string
		partition func([]int, int, int) int
	}{
		{"DoublePointerSwap", DoublePointerSwap},
		{"DoublePointerHole", DoublePointerHole},
		{"SinglePointer", SinglePointer},
	} {
		got := slices.Clone(arr)
		QuickSortBy(got, 0, len(got)-1, s.partition)
		if !slices.Equal(got, want) {
			t.Fatalf("%s mismatch with sort.Slice", s.name)
		}
	}
}

func TestMenuTreeSort(t *testing.T) {
	// 与 Java NumberSort.main 完全一致的 14 个节点（故意乱序添加）
	menus := []Menu{
		{ID: "41362f5f-bcba-11e8-98b0-00163e0278b2", ParentID: "zhengGaiZiLiaoKuYiJiId", Name: "相关政策", SerialNumber: 1},
		{ID: "41783287-bcba-11e8-98b0-00163e0278b2", ParentID: "zhengCeZiLiaoKuYiJiId", Name: "国家政策", SerialNumber: 1},
		{ID: "6afe2621-9ff9-4dcf-af5e-c14af5793c2c", ParentID: "a635d68f-338c-49f9-8aec-b3bee6ea8d2d", Name: "教师个人资料", SerialNumber: 1},
		{ID: "XUEXIAOWENHUAKU", Name: "学校文化库", SerialNumber: 1},
		{ID: "ac371b82-bdb5-41ce-8741-973be4c6fc1d", ParentID: "XUEXIAOWENHUAKU", Name: "音乐文化库", SerialNumber: 1},
		{ID: "41a3b305-bcba-11e8-98b0-00163e0278b2", ParentID: "zhengCeZiLiaoKuYiJiId", Name: "省政策", SerialNumber: 2},
		{ID: "58e89703-7919-11e9-9079-00163e0278b2", ParentID: "zhengGaiZiLiaoKuYiJiId", Name: "方法与工具", SerialNumber: 2},
		{ID: "71265b40-4cd9-4ce1-ab09-8ed5aaf95822", ParentID: "XUEXIAOWENHUAKU", Name: "体育健身", SerialNumber: 2},
		{ID: "XUEXIAOZHIDUKU", Name: "学校制度库", SerialNumber: 2},
		{ID: "41b05c9f-bcba-11e8-98b0-00163e0278b2", ParentID: "zhengCeZiLiaoKuYiJiId", Name: "市政策", SerialNumber: 3},
		{ID: "zhengGaiZiLiaoKuYiJiId", Name: "诊改资料库", SerialNumber: 3},
		{ID: "zhengCeZiLiaoKuYiJiId", Name: "政策资料库", SerialNumber: 4},
		{ID: "80d178fd-938c-48c3-b6f9-5514b7a7fffd", Name: "测试库", SerialNumber: 5},
		{ID: "a635d68f-338c-49f9-8aec-b3bee6ea8d2d", Name: "学校资料库", SerialNumber: 6},
	}

	got := SortMenus(menus)
	wantIDs := []string{
		"XUEXIAOWENHUAKU",                      // 学校文化库（根 1）
		"ac371b82-bdb5-41ce-8741-973be4c6fc1d", // 音乐文化库
		"71265b40-4cd9-4ce1-ab09-8ed5aaf95822", // 体育健身
		"XUEXIAOZHIDUKU",                       // 学校制度库（根 2）
		"zhengGaiZiLiaoKuYiJiId",               // 诊改资料库（根 3）
		"41362f5f-bcba-11e8-98b0-00163e0278b2", // 相关政策
		"58e89703-7919-11e9-9079-00163e0278b2", // 方法与工具
		"zhengCeZiLiaoKuYiJiId",                // 政策资料库（根 4）
		"41783287-bcba-11e8-98b0-00163e0278b2", // 国家政策
		"41a3b305-bcba-11e8-98b0-00163e0278b2", // 省政策
		"41b05c9f-bcba-11e8-98b0-00163e0278b2", // 市政策
		"80d178fd-938c-48c3-b6f9-5514b7a7fffd", // 测试库（根 5）
		"a635d68f-338c-49f9-8aec-b3bee6ea8d2d", // 学校资料库（根 6）
		"6afe2621-9ff9-4dcf-af5e-c14af5793c2c", // 教师个人资料
	}

	if len(got) != len(wantIDs) {
		t.Fatalf("结果 %d 个节点, want %d：实际序 %s", len(got), len(wantIDs), idChain(got))
	}
	for i := range wantIDs {
		if got[i].ID != wantIDs[i] {
			t.Fatalf("第 %d 个应为 %s，实际 %s；全序：%s", i, wantIDs[i], got[i].ID, idChain(got))
		}
	}
}

// idChain 失败时打印整条 DFS 序，方便排查。
func idChain(menus []Menu) string {
	ids := make([]string, len(menus))
	for i, m := range menus {
		ids[i] = m.ID
	}
	return strings.Join(ids, " → ")
}

func TestQueueStackMirrorsJavaMain(t *testing.T) {
	// 逐句镜像 Java ListStack.main：push 1,2 → pop=2 → push 3 → pop=3 → 剩 [1] → pop=1 → 空
	var s QueueStack[string]
	s.Push("1")
	s.Push("2")
	if v, ok := s.Pop(); !ok || v != "2" {
		t.Fatalf("第一次 pop: %q %v, want 2", v, ok)
	}
	s.Push("3")
	if v, ok := s.Pop(); !ok || v != "3" {
		t.Fatalf("第二次 pop: %q %v, want 3", v, ok)
	}
	if got := s.String(); got != "[1]" { // 对应 Java 最后 println 前队列里只剩 1
		t.Fatalf("两次 pop 后应剩 [1]，实际 %q", got)
	}
	if v, ok := s.Pop(); !ok || v != "1" {
		t.Fatalf("第三次 pop: %q %v, want 1", v, ok)
	}
	if got := s.String(); got != "[]" {
		t.Fatalf("全部弹空后应为 []，实际 %q", got)
	}
	if _, ok := s.Pop(); ok { // Java 空栈 pop 返回 null；Go 返回 (零值, false)
		t.Fatal("空栈 pop 应返回 ok=false")
	}
}

func TestSliceStack(t *testing.T) {
	var s SliceStack[int]
	for i := 1; i <= 3; i++ {
		s.Push(i * 10)
	}
	if got := s.String(); got != "[10,20,30]" {
		t.Fatalf("String: %q", got)
	}
	for want := 30; want >= 10; want -= 10 {
		if v, ok := s.Pop(); !ok || v != want {
			t.Fatalf("pop: %d %v, want %d", v, ok, want)
		}
	}
	if _, ok := s.Pop(); ok {
		t.Fatal("空栈 pop 应返回 ok=false")
	}
}

func TestFrogJump(t *testing.T) {
	// 三版本在 n=10 处一致：1,1,2,3,5,8,13,21,34,55,89 → f(10)=89
	if got := FrogJumpCommon(10); got != 89 {
		t.Fatalf("朴素版 f(10)=%d, want 89", got)
	}
	if got := FrogJump(10, []int64{1, 2}); got != 89 {
		t.Fatalf("记忆化版 f(10)=%d, want 89", got)
	}
	if got := FrogJumpRolling(10); got != 89 {
		t.Fatalf("滚动版 f(10)=%d, want 89", got)
	}

	// 任意步长：steps={1,2,3} → 1,1,2,4,7（Tribonacci 变形），f(4)=7
	if got := FrogJump(4, []int64{1, 2, 3}); got != 7 {
		t.Fatalf("steps={1,2,3} f(4)=%d, want 7", got)
	}

	// 边界：f(0)=f(1)=1
	if FrogJump(0, []int64{1, 2}) != 1 || FrogJumpRolling(0) != 1 {
		t.Fatal("f(0) 应为 1")
	}

	// 安全边界精确值：f(90) = 4660046610375530309（Java 版算 f(100) 已静默溢出）
	const want90 = int64(4660046610375530309)
	if got := FrogJump(90, []int64{1, 2}); got != want90 {
		t.Fatalf("f(90)=%d, want %d", got, want90)
	}
	if got := FrogJumpRolling(90); got != want90 {
		t.Fatalf("滚动版 f(90)=%d, want %d", got, want90)
	}

	// 超界应 panic（显式防御，而不是静默溢出）
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("n=91 应该 panic 防溢出")
			}
		}()
		FrogJump(91, []int64{1, 2})
	}()
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("n=91 应该 panic 防溢出")
			}
		}()
		FrogJumpRolling(91)
	}()
}
