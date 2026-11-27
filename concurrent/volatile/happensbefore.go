package volatile

import "sync"

// 本文件对应 Java ReOrder（JSR-133 经典场景：r2=A; B=1 / r1=B; A=2，20 万次循环数
// "惊喜"次数）—— 但它不能被确定性复现，原因本身就是教学重点。
//
// Java ReOrder 演示的是"无同步共享变量下，编译器/CPU 重排让线程看到不可能的因果顺序"。
// Go 同样会重排（编译器 + CPU 都会），但 Go 的应对不是关键字，而是【契约 + 检查器】：
//
//   - 契约：Go 内存模型定义了 happens-before。channel 发送 happens-before 对应接收
//     完成；Mutex Unlock happens-before 后续 Lock；atomic 写 happens-before atomic 读。
//     在没有 happens-before 边的代码里看到"乱序"结果不叫 bug，叫未同步 —— Java 的
//     volatile 在 Go 里的身份就是这些边。
//   - 检查器：go test -race 把所有"缺少 happens-before 边的并发访问"点名报出来。
//
// 所以 ReOrder 的 Go 版不数惊喜次数（概率型输出对自动化测试不友好，且裸共享变量
// 本身就是 race），而是验证【契约成立】：下面两个演示在有 happens-before 边的通道上，
// 读到的因果顺序必须正确 —— race 下也能跑。
//
// ChannelHappensBefore 写方先写数据再发 channel；读方接收后读数据。
// 若没有"发送 happens-before 接收"这条边，42 可能读不到（对应 Java r2=A 读到 0）。
func ChannelHappensBefore() int {
	data := 0
	ch := make(chan struct{})
	go func() {
		data = 42 // 普通写
		close(ch) // 发送（close 也是发送形态）
	}()
	<-ch // 接收：与上面的发送建立 happens-before 边
	return data
}

// MutexHappensBefore 用 Mutex 建立 同样的边：Unlock happens-before 后续 Lock。
// 对应 Java volatile 的读写对 —— Java 用 volatile 传递可见性，Go 用锁/channel/atomic
// 三种原语都能传递，选一个顺手的。
func MutexHappensBefore() int {
	var mu sync.Mutex
	data := 0
	done := make(chan struct{})
	go func() {
		mu.Lock()
		data = 42
		mu.Unlock()
		close(done)
	}()
	<-done
	mu.Lock()
	v := data // 与写 goroutine 的 Unlock 有 happens-before 边
	mu.Unlock()
	return v
}

// HappensBeforeContract 一次性验证两条契约边（测试入口）。
func HappensBeforeContract() (viaChannel, viaMutex int) {
	return ChannelHappensBefore(), MutexHappensBefore()
}
