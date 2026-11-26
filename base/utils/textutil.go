package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Verbs fmt 动词速查（对应 Java String.format 的 %d/%s/%n）：
//
//	%v    默认格式：切片 [1 2 3]（相当于 Java Arrays.toString）
//	%T    类型名（Go 是静态类型 + 推断，打印类型看这个最直接）
//	%d    十进制整数；%05d 宽度 5 补零（对照 Java 的 %05d）
//	%q    带引号字符串：不可见字符被转义显示，调试输出特别有用
//	%.2f  两位小数（对照 Java 的 %.2f）
func Verbs() (slice, typeName, padded, quoted, fixed string) {
	slice = fmt.Sprintf("%v", []int{1, 2, 3}) // "[1 2 3]" —— 注意没有逗号
	typeName = fmt.Sprintf("%T", 3.14)        // "float64"
	padded = fmt.Sprintf("%05d", 42)          // "00042"
	quoted = fmt.Sprintf("%q", "你好\tGo")      // `"你好\tGo"` —— 制表符转义成 \t
	fixed = fmt.Sprintf("%.2f", 3.14159)      // "3.14"
	return
}

// Atoi 字符串 → 整数（对照 Java Integer.parseInt）。
// Go 惯例：解析失败返回 (0, err)，错误必须处理，没有 Java 的 NumberFormatException 往外抛。
func Atoi(s string) (int, error) {
	return strconv.Atoi(s)
}

// FormatInt 整数 → 字符串（对照 String.valueOf(int)）。
func FormatInt(n int) string {
	return strconv.Itoa(n)
}

// ParseFloat 字符串 → 浮点（对照 Double.parseDouble）。位宽显式指定：32 或 64。
func ParseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

// StringsDemo strings 包高频函数（对应 Java String 的一堆方法 + String.join）。
func StringsDemo() (joined string, parts []string, hasPrefix, contains bool, replaced string) {
	joined = strings.Join([]string{"go", "is", "fun"}, "-") // "go-is-fun"（对照 String.join）
	parts = strings.Split("a,b,c", ",")                     // ["a" "b" "c"]（对照 split，正则语义的 split 注意转义）
	hasPrefix = strings.HasPrefix("HelloGo", "Hello")       // true（对照 startsWith）
	contains = strings.Contains("HelloGo", "loG")           // true（对照 contains）
	replaced = strings.ReplaceAll("2026/11/26", "/", "-")   // "2026-11-26"（对照 replace）
	return
}

// dateRe 编译期包级正则（Must*：写错直接 panic —— 正则是程序员的错，尽早炸）。
// 对照 Java：Pattern.compile + Matcher。
// 差异：Go 用 RE2 引擎，线性时间、无回溯 —— 不支持反向引用，但也杜绝了灾难性回溯。
var dateRe = regexp.MustCompile(`(\d{4})-(\d{2})-(\d{2})`)

// FindDate 提取文本中第一个 yyyy-MM-dd 日期（对照 matcher.find() + group()）。
func FindDate(text string) string {
	return dateRe.FindString(text)
}

// FindDateGroups 提取日期的年/月/日子分组（对照 group(1)/group(2)/group(3)）。
func FindDateGroups(text string) (year, month, day string, ok bool) {
	m := dateRe.FindStringSubmatch(text)
	if m == nil {
		return "", "", "", false
	}
	return m[1], m[2], m[3], true
}
