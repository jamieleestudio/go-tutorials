package structure

// MapBasics map 的读与删：comma-ok 双返回值是关键习惯。
// 对应 Java 的 HashMap：get 返回 null 需判空；Go 不存在取不到时的异常，只给 (零值, false)。
func MapBasics() (found int, okFound bool, missing int, okMissing bool) {
	m := map[string]int{"apple": 1, "banana": 2}
	found, okFound = m["apple"]      // 1, true
	missing, okMissing = m["cherry"] // 0, false —— 零值兜底，不抛异常
	delete(m, "apple")               // 删除不存在的 key 也不报错
	return
}

// Counting 统计词频的惯用法。
// 对照 Java：map.merge(w, 1, Integer::sum) —— Go 的零值语义让 ++ 直接可用。
func Counting(words []string) map[string]int {
	counts := map[string]int{}
	for _, w := range words {
		counts[w]++
	}
	return counts
}

// MapIsReference map 是引用类型：赋值共享同一份数据（对应 Java 的引用语义）。
func MapIsReference() (viaM1, viaM2 int) {
	m1 := map[string]int{"k": 1}
	m2 := m1
	m2["k"] = 99
	return m1["k"], m2["k"]
}

// AnyValue 值类型任意：map[string]any 对应 Map<String, Object>。
// range map 的顺序是随机的（运行时故意打乱，防止依赖顺序的代码）。
func AnyValue() map[string]any {
	return map[string]any{
		"name": "Go",
		"year": 2009,
		"tags": []string{"并发", "简洁"},
		"ok":   true,
	}
}
