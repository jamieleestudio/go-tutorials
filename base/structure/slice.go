// Package structure 数据结构课：切片、map、struct、接口、泛型与"嵌套类"的 Go 对应物。
//
// 对应 java-tutorials 的 base/se/structure 与 Java 集合框架/OOP 心智模型的迁移。
package structure

// ArrayIsValue 数组是值类型：赋值/传参发生整体拷贝。
// 对照 Java：数组是引用，Go 的数组几乎总是作为切片的底层数据出现，直接用的场景很少。
func ArrayIsValue() (a, b [3]int) {
	a = [3]int{1, 2, 3}
	b = a // 完整拷贝
	b[0] = 99
	return
}

// SliceBasics 切片 = {指向底层数组的指针, len, cap} 的"视图"结构。
// 对应 Java 的 ArrayList（可变长度），但注意：多个切片可以共享同一底层数组。
func SliceBasics() (s []int, length, capacity int) {
	s = make([]int, 3, 5) // len=3 cap=5
	s[0], s[1], s[2] = 1, 2, 3
	s = append(s, 4) // cap 内追加：不搬家
	return s, len(s), cap(s)
}

// SliceAliasing 陷阱课：子切片与父切片【共享底层数组】——
//   - 修改子切片元素 → 父切片"看得见"；
//   - cap 内 append → 直接写进父切片还没用到的位置。
//
// 对应 Java：subList 也是视图，但没有"append 写穿父集合"这种隐蔽行为。
func SliceAliasing() (s1 []int) {
	s1 = []int{1, 2, 3, 4, 5}
	sub := s1[1:3]        // len=2 cap=4
	sub[0] = 99           // 写穿：s1[1] 变 99
	sub = append(sub, 88) // cap 未超：写进 s1[3]
	_ = sub
	return
}

// SliceFullExpr 三索引切片 s[low:high:max]：把 cap 钉死，append 必然搬家 —— 杜绝写穿。
func SliceFullExpr() (s1 []int) {
	s1 = []int{1, 2, 3, 4, 5}
	sub := s1[1:3:3] // cap 限制到 3（相对底层数组起点）
	sub = append(sub, 100)
	_ = sub
	return
}

// CopySlice copy 显式复制 —— 对应 Java 的 new ArrayList<>(src)。
func CopySlice() (src, dst []int) {
	src = []int{1, 2, 3}
	dst = make([]int, len(src))
	copy(dst, src)
	dst[0] = 99
	return
}

// DeleteElem 删除中间元素的惯用法（保持顺序）—— 后半段整体前移。
func DeleteElem(s []int, i int) []int {
	return append(s[:i], s[i+1:]...) // ... 把切片展开成可变参数
}

// NilVsEmpty nil 切片与空切片：len/cap 都是 0，append 都可用。
// 惯例：nil 表示"还没有数据"；API 一般不必区分两者。
func NilVsEmpty() (okNil, okEmpty bool) {
	var nilSlice []int
	empty := []int{}
	nilSlice = append(nilSlice, 1)
	empty = append(empty, 1)
	return len(nilSlice) == 1, len(empty) == 1
}
