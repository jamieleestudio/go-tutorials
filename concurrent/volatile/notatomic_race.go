//go:build race

// race 构建下的占位实现 —— 这本身是一个教学点：
// 同一个"丢更新演示"在两种构建下给出两种答案：
//
//	go test        （!race）→ 真实数据竞争，静默丢更新，got < want
//	go test -race  （race） → 竞争检测器在 PlainCounterBroken 里当场报 DATA RACE，
//	                          测试还没跑完就失败 —— Java 版要到"输出 lost > 0"才发现问题，
//	                          Go 版是写代码的当下就被点名。
//
// 所以 race 构建下不能执行真竞争代码，返回占位值让测试只验证"修复版精确无误"。
package volatile

// PlainCounterBroken 占位实现（race 构建下被测到时直接返回精确值）。
func PlainCounterBroken(goroutines, iters int) (got, want int) {
	return goroutines * iters, goroutines * iters
}
