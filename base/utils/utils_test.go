package utils

import (
	"strings"
	"testing"
	"time"
)

func TestVerbs(t *testing.T) {
	slice, typeName, padded, quoted, fixed := Verbs()
	if slice != "[1 2 3]" {
		t.Fatalf("%%v: %q, want [1 2 3]", slice)
	}
	if typeName != "float64" {
		t.Fatalf("%%T: %q, want float64", typeName)
	}
	if padded != "00042" {
		t.Fatalf("%%05d: %q, want 00042", padded)
	}
	if quoted != `"你好\tGo"` { // %q 把制表符转义显示
		t.Fatalf("%%q: %q", quoted)
	}
	if fixed != "3.14" {
		t.Fatalf("%%.2f: %q, want 3.14", fixed)
	}
}

func TestStrconv(t *testing.T) {
	n, err := Atoi("42")
	if err != nil || n != 42 {
		t.Fatalf("Atoi: %d %v", n, err)
	}
	if _, err := Atoi("abc"); err == nil {
		t.Fatal("Atoi(abc) 应返回错误")
	}
	if got := FormatInt(42); got != "42" {
		t.Fatalf("FormatInt: %q", got)
	}
	f, err := ParseFloat("3.5")
	if err != nil || f != 3.5 {
		t.Fatalf("ParseFloat: %v %v", f, err)
	}
}

func TestStringsDemo(t *testing.T) {
	joined, parts, hasPrefix, contains, replaced := StringsDemo()
	if joined != "go-is-fun" {
		t.Fatalf("Join: %q", joined)
	}
	if strings.Join(parts, "|") != "a|b|c" || len(parts) != 3 {
		t.Fatalf("Split: %v", parts)
	}
	if !hasPrefix || !contains {
		t.Fatalf("HasPrefix/Contains: %v %v", hasPrefix, contains)
	}
	if replaced != "2026-11-26" {
		t.Fatalf("ReplaceAll: %q", replaced)
	}
}

func TestFindDate(t *testing.T) {
	text := "公告发布于 2026-11-26，更新于 2027-01-01。"
	if got := FindDate(text); got != "2026-11-26" {
		t.Fatalf("FindDate: %q, want 2026-11-26", got)
	}
	year, month, day, ok := FindDateGroups(text)
	if !ok || year != "2026" || month != "11" || day != "26" {
		t.Fatalf("FindDateGroups: %q %q %q %v", year, month, day, ok)
	}
	if _, _, _, ok := FindDateGroups("没有日期的文本"); ok {
		t.Fatal("无匹配应返回 ok=false")
	}
}

func TestTimeLayout(t *testing.T) {
	// ParseTrap：直接 Parse 会丢时区（按 UTC 处理）
	trap := ParseTrap("2026-11-26 08:30:00")
	if trap.Location() != time.UTC {
		t.Fatalf("time.Parse 应按 UTC 处理：%v", trap.Location())
	}

	// ParseFixed：ParseInLocation 保住本地时区，墙上时钟不变
	got, err := ParseFixed("2026-11-26 08:30:00")
	if err != nil {
		t.Fatal(err)
	}
	if got.Hour() != 8 || got.Minute() != 30 || got.Location() != time.Local {
		t.Fatalf("ParseFixed: %v @ %v, hour=%d", got, got.Location(), got.Hour())
	}
	if FormatFixed(got) != "2026-11-26 08:30:00" {
		t.Fatalf("FormatFixed 往返不一致: %q", FormatFixed(got))
	}
}

func TestAddDate(t *testing.T) {
	base := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)

	plus5 := AddDays(base, 5)
	if plus5.Month() != time.December || plus5.Day() != 1 {
		t.Fatalf("AddDays(5): %v, want 2026-12-01", plus5.Format(LayoutDate))
	}

	plus2m := AddMonths(base, 2)
	if plus2m.Month() != time.January || plus2m.Day() != 26 || plus2m.Year() != 2027 {
		t.Fatalf("AddMonths(2): %v, want 2027-01-26", plus2m.Format(LayoutDate))
	}
}

func TestBetweenDays(t *testing.T) {
	a := time.Date(2026, 11, 26, 23, 0, 0, 0, time.UTC) // 带上时分秒证明"按自然日"计算
	b := time.Date(2026, 12, 25, 1, 0, 0, 0, time.UTC)
	if got := BetweenDays(a, b); got != 29 {
		t.Fatalf("BetweenDays: %d, want 29", got)
	}
}
