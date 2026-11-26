// Package algorithm 算法练习：与 java-tutorials 的 base/se/algorithm 同名同题 ——
// QuickSort（三种 partition）、NumberSort（树形菜单排序）、ListStack（队列实现栈）、FrogJump（跳台阶）。
package algorithm

// QuickSort 快速排序入口：分而治之 —— 先 partition 定住一个元素的最终位置，再递归左右区间。
//
// 对照 Java：quickSort(int[] arr, int startIndex, int endIndex)。Go 的切片是"视图"，
// 就地修改直接生效（Java 数组同理），所以函数不需要返回值。
func QuickSort(arr []int, startIndex, endIndex int) {
	if startIndex >= endIndex {
		return
	}

	// 三种分治实现可互换。Java 版靠注释切换，Go 版做成显式可传的策略函数（见 QuickSortBy）：
	pivotIndex := DoublePointerSwap(arr, startIndex, endIndex)
	// pivotIndex := DoublePointerHole(arr, startIndex, endIndex)
	// pivotIndex := SinglePointer(arr, startIndex, endIndex)

	// 用分界值下标分成两个区间，分别递归处理
	QuickSort(arr, startIndex, pivotIndex-1)
	QuickSort(arr, pivotIndex+1, endIndex)
}

// QuickSortBy 用调用方指定的 partition 策略排序 —— Java 版注释切换的"函数式"版。
// 对照 Java：传入 Comparator/函数式接口；Go 直接传函数值。
func QuickSortBy(arr []int, startIndex, endIndex int, partition func([]int, int, int) int) {
	if startIndex >= endIndex {
		return
	}
	pivotIndex := partition(arr, startIndex, endIndex)
	QuickSortBy(arr, startIndex, pivotIndex-1, partition)
	QuickSortBy(arr, pivotIndex+1, endIndex, partition)
}

// DoublePointerSwap 双边指针（交换法）：pivot 取首元素；
// 右指针从右往左找"比 pivot 小"的，左指针从左往右找"比 pivot 大"的，
// 没有相遇就交换两者；相遇后把 pivot 换到相遇点 —— 相遇点左侧 ≤ pivot，右侧 > pivot。
func DoublePointerSwap(arr []int, startIndex, endIndex int) int {
	pivot := arr[startIndex]
	leftPoint, rightPoint := startIndex, endIndex

	for leftPoint < rightPoint {
		// 右指针从右往左找比 pivot 小的数据
		for leftPoint < rightPoint && arr[rightPoint] > pivot {
			rightPoint--
		}
		// 左指针从左往右找比 pivot 大的数据
		for leftPoint < rightPoint && arr[leftPoint] <= pivot {
			leftPoint++
		}
		// 没有相遇则交换
		if leftPoint < rightPoint {
			arr[leftPoint], arr[rightPoint] = arr[rightPoint], arr[leftPoint] // Go 的多赋值交换，不需要 temp
		}
	}
	// 最终将分界值与当前指针所在数据交换
	arr[startIndex] = arr[rightPoint]
	arr[rightPoint] = pivot
	// 返回分界值所在下标
	return rightPoint
}

// DoublePointerHole 双边指针（挖坑法）：pivot 先"挖走"首元素形成坑；
// 右指针找到小的填进左边的坑（该处变成新坑），左指针找到大的填进右边的坑；
// 循环结束把最初记录的 pivot 填进最后的坑。
func DoublePointerHole(arr []int, startIndex, endIndex int) int {
	pivot := arr[startIndex]
	leftPoint, rightPoint := startIndex, endIndex

	for leftPoint < rightPoint {
		// 右指针从右往左找比 pivot 小的数据
		for leftPoint < rightPoint && arr[rightPoint] > pivot {
			rightPoint--
		}
		// 找到后填入左边的坑，当前位置变为新的"坑"
		if leftPoint < rightPoint {
			arr[leftPoint] = arr[rightPoint]
			leftPoint++
		}
		// 左指针从左往右找比 pivot 大的数据
		for leftPoint < rightPoint && arr[leftPoint] <= pivot {
			leftPoint++
		}
		// 找到后填入右边的坑，当前位置变为新的"坑"
		if leftPoint < rightPoint {
			arr[rightPoint] = arr[leftPoint]
			rightPoint--
		}
	}
	// 最开始存储的分界值填入当前的"坑"中
	arr[rightPoint] = pivot
	return rightPoint
}

// SinglePointer 单指针法：mark 是"小于 pivot 区域"的右边界；
// 顺序扫一遍，扫到比 pivot 小的就把该区扩大一格（mark++ 后互换）；
// 循环结束后把首元素 pivot 与 mark 处元素互换。
func SinglePointer(arr []int, startIndex, endIndex int) int {
	pivot := arr[startIndex]
	markPoint := startIndex

	for i := startIndex + 1; i <= endIndex; i++ {
		// 比分界值小的，则 mark++ 后互换
		if arr[i] < pivot {
			markPoint++
			arr[markPoint], arr[i] = arr[i], arr[markPoint]
		}
	}
	// 首元素分界值与当前 mark 所在数据互换
	arr[startIndex] = arr[markPoint]
	arr[markPoint] = pivot
	return markPoint
}
