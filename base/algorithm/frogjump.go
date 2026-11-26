package algorithm

// FrogJumpCommon 朴素递归版（对应 Java frogJumpCommon）：
// f(n) = f(n-1) + f(n-2)，f(0)=f(1)=1 —— 即斐波那契变形（一次跳 1 或 2 阶）。
// 指数级时间：大量子问题被重复计算，只适合演示。
func FrogJumpCommon(n int64) int64 {
	if n < 2 {
		return 1
	}
	return FrogJumpCommon(n-1) + FrogJumpCommon(n-2)
}

// FrogJump 记忆化版（对应 Java frogJump(long, long[], Map)）：
// 任意步长集合 steps，每个子问题只算一次 —— O(n × len(steps)) 时间。
//
// 溢出警示：Java 版 main 里跑 f(100, {1,2}) 其实已经静默溢出 long
// （真值 ≈ 5.7e20 > 2^63-1 ≈ 9.2e18）；Go 的 int64 一样会静默溢出。
// 所以教学版显式设防：n > 90 直接 panic（f(90) = 4660046610375530309 是安全边界，
// f(92) 就越界了）。
func FrogJump(totalSteps int64, steps []int64) int64 {
	if totalSteps > 90 {
		panic("f(n) 超出 int64 范围：教学版仅支持 n ≤ 90（Java 版算 f(100) 已静默溢出）")
	}

	cache := make(map[int64]int64)
	var f func(int64) int64
	f = func(n int64) int64 {
		if v, ok := cache[n]; ok {
			return v
		}
		if n < 2 {
			return 1
		}
		var total int64
		for _, step := range steps {
			if n >= step {
				total += f(n - step)
			}
		}
		cache[n] = total
		return total
	}
	return f(totalSteps)
}

// FrogJumpRolling 滚动变量版 —— 题目的"问题 1 / 问题 2"：
// f(n) 只依赖最近两个状态，滚动保留即可：O(n) 时间、O(1) 空间。
// 对照 Java：没有现成对应，这是动态规划的标准手法。
func FrogJumpRolling(n int64) int64 {
	if n > 90 {
		panic("f(n) 超出 int64 范围：教学版仅支持 n ≤ 90")
	}
	if n < 2 {
		return 1
	}
	prev, curr := int64(1), int64(1) // f(0), f(1)
	for i := int64(2); i <= n; i++ {
		prev, curr = curr, prev+curr
	}
	return curr
}
