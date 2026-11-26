// Package utils 工具函数：时间与文本 —— 对应 java-tutorials base/se/utils 的
// DateConstants/DateUtils/DateUtils2，以及 Java 里散落的 String.format 与正则练习。
package utils

import "time"

// Layout Go 的时间排版不用 "yyyy-MM-dd HH:mm:ss" 模式串，而是"参考时间"：
// 2006-01-02 15:04:05 正是 Mon Jan 2 15:04:05 MST 2006（Unix 纪元参考瞬间）逐位对应 ——
// 1月2日3点4分5秒2006年。记不住就记这个梗。
// 对应 Java DateConstants 里的常量。
const Layout = "2006-01-02 15:04:05"

// LayoutDate 日期专用排版。
const LayoutDate = "2006-01-02"

// ParseTrap 对照组（误区演示）：不带位置信息直接 time.Parse，字符串会被当成 UTC ——
// 用本地时区语义解析时间时差 8 小时（北京时间经典坑）。
func ParseTrap(s string) time.Time {
	t, err := time.Parse(Layout, s)
	if err != nil {
		panic(err)
	}
	return t
}

// ParseFixed 正确姿势：ParseInLocation 指定按本地时区解析，墙上时钟不变。
// 对照 Java：LocalDateTime.parse(s, DateTimeFormatter.ofPattern(...))。
func ParseFixed(s string) (time.Time, error) {
	return time.ParseInLocation(Layout, s, time.Local)
}

// FormatFixed 格式化（对照 Java SimpleDateFormat.format / DateTimeFormatter.format）。
func FormatFixed(t time.Time) string {
	return t.Format(Layout)
}

// AddDays 加 N 天（对照 Java LocalDateTime.plusDays）。
func AddDays(t time.Time, days int) time.Time {
	return t.AddDate(0, 0, days)
}

// AddMonths 加 N 个月（对照 plusMonths）。教学点：AddDate 做溢出归一化，
// 1 月 31 日 + 1 个月 = 3 月 3 日（2 月没有 31 日，顺延）—— 与 Java 不同，
// Java plusMonths 会截断到月末。跨月计算要留意。
func AddMonths(t time.Time, months int) time.Time {
	return t.AddDate(0, months, 0)
}

// BetweenDays 两个时刻之间隔的自然日数（各自截断到当日零点再相减）。
// 教学点：先想清楚要"绝对时长"还是"自然日差"——这里是后者。
func BetweenDays(a, b time.Time) int {
	aDay := time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, a.Location())
	bDay := time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, b.Location())
	return int(bDay.Sub(aDay).Hours() / 24)
}
