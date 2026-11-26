package algorithm

import (
	"fmt"
	"strings"
)

// QueueStack 用队列实现栈 —— Java ListStack<T>（ArrayBlockingQueue 版）的镜像：
// push 直接入队尾；pop 把前 n-1 个元素出队再入队（轮转一圈），最后出队的那个就是"栈顶"。
//
// 教学点：一个队列就够了（Java 版就是这么写的）；入栈 O(1)、出栈 O(n)。
// 对照 Java 泛型 class ListStack<T>：Go 用 [T any] 泛型类型。
// 差异：Java 的 pop 空栈返回 null；Go 没有 null —— 返回 (零值, false)，
// 调用方被迫处理空栈情况，比 null 安全。
type QueueStack[T any] struct {
	queue []T // Go 惯用切片模拟队列（出队=截头，入队=append）
}

// Push 入栈：直接追加到队尾（对应 queue.add）。
func (s *QueueStack[T]) Push(v T) {
	s.queue = append(s.queue, v)
}

// Pop 出栈：轮转 len-1 次，让栈顶排到队头，再出队它。
func (s *QueueStack[T]) Pop() (T, bool) {
	var zero T
	n := len(s.queue)
	if n == 0 {
		return zero, false
	}
	for range n - 1 {
		s.queue = append(s.queue[1:], s.queue[0]) // 出队 + 入队 = 轮转
	}
	top := s.queue[0]
	s.queue = s.queue[1:]
	return top, true
}

// String 实现 fmt.Stringer（对应 Java 的 toString + StringJoiner）。
func (s *QueueStack[T]) String() string {
	parts := make([]string, len(s.queue))
	for i, v := range s.queue {
		parts[i] = fmt.Sprint(v)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// SliceStack Go 惯用版：切片天然就是栈 —— append 入栈、截尾出栈，入栈出栈都是 O(1)，
// 没有轮转开销。教学点："队列轮转实现栈"是面试题；生产代码永远写这一版
// （对应 Java 的 Deque/ArrayDeque）。
type SliceStack[T any] struct {
	items []T
}

func (s *SliceStack[T]) Push(v T) { s.items = append(s.items, v) }

func (s *SliceStack[T]) Pop() (T, bool) {
	var zero T
	if len(s.items) == 0 {
		return zero, false
	}
	top := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1] // 截尾：len 减一，底层数组复用
	return top, true
}

func (s *SliceStack[T]) String() string {
	parts := make([]string, len(s.items))
	for i, v := range s.items {
		parts[i] = fmt.Sprint(v)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
