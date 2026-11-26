package algorithm

import "slices"

// Menu 菜单节点（对应 Java NumberSort.Menu，Lombok @Getter/@Setter 在 Go 里就是导出字段）。
// SerialNumber 是同级排序号；Go 没有 null —— ParentID=="" 即"根节点"（对应 Java 的 null）。
type Menu struct {
	ID           string
	ParentID     string
	Name         string
	SerialNumber int
}

// SortMenus 树形 DFS 排序：先按 SerialNumber 排根，再对每个根递归下钻 ——
// 每一层内部都按 SerialNumber 排序，输出即整棵菜单树的"前序遍历"。
//
// 对照 Java：menus.stream().filter(parentId).sorted(comparing(serialNumber)) 的递归版。
// 迁移点：
//   - Java 的 null → Go 的零值 ""；
//   - Stream 链 → 显式 for 循环 + slices.SortFunc；
//   - 输入顺序无关：结果只由 (ParentID, SerialNumber) 决定。
func SortMenus(menus []Menu) []Menu {
	result := make([]Menu, 0, len(menus))

	roots := make([]Menu, 0)
	for _, m := range menus {
		if m.ParentID == "" {
			roots = append(roots, m)
		}
	}
	slices.SortFunc(roots, bySerial)

	for _, root := range roots {
		buildTreeSortList(root, menus, &result)
	}
	return result
}

// buildTreeSortList 前序遍历：先记录当前节点，再递归每个（按序号排好的）孩子。
func buildTreeSortList(current Menu, all []Menu, result *[]Menu) {
	*result = append(*result, current)

	children := make([]Menu, 0)
	for _, m := range all {
		if m.ParentID == current.ID {
			children = append(children, m)
		}
	}
	slices.SortFunc(children, bySerial)

	for _, child := range children {
		buildTreeSortList(child, all, result)
	}
}

// bySerial 同级排序的比较函数（对应 Java Comparator.comparing(Menu::getSerialNumber)）。
func bySerial(a, b Menu) int {
	return a.SerialNumber - b.SerialNumber
}
