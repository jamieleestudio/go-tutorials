package structure

// Number 约束：类型集合（对应 Java 的 <T extends Number>）。
// 关键差异：Go 泛型没有通配符（? extends / ? super），约束本身就是接口。
type Number interface {
	~int | ~int64 | ~float64 // ~ 表示"底层类型为…"的自定义类型也算
}

// Sum 泛型函数。
// 对照 Java：static <T extends Number> long sum(List<T> list)。
func Sum[T Number](nums []T) T {
	var total T
	for _, n := range nums {
		total += n
	}
	return total
}

// Map / Filter / Reduce 泛型三件套 —— 对应 Java Stream 的同名操作，
// 只是 Stream 是链式流，Go 是显式函数组合（无惰性求值）。
func Map[T, R any](in []T, f func(T) R) []R {
	out := make([]R, 0, len(in))
	for _, v := range in {
		out = append(out, f(v))
	}
	return out
}

func Filter[T any](in []T, keep func(T) bool) []T {
	out := make([]T, 0, len(in))
	for _, v := range in {
		if keep(v) {
			out = append(out, v)
		}
	}
	return out
}

func Reduce[T, R any](in []T, init R, f func(R, T) R) R {
	acc := init
	for _, v := range in {
		acc = f(acc, v)
	}
	return acc
}

// Stack 泛型类型（对应 Java 的 class Stack<T>）。
type Stack[T any] struct {
	items []T
}

func (s *Stack[T]) Push(v T) { s.items = append(s.items, v) }

func (s *Stack[T]) Pop() (T, bool) {
	var zero T
	if len(s.items) == 0 {
		return zero, false
	}
	top := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return top, true
}

func (s *Stack[T]) Len() int { return len(s.items) }
