package foundation

import (
	"errors"
	"fmt"
)

// ErrNotFound 哨兵错误：包级预定义，调用方用 errors.Is 判断（对应 Java 按异常类型 catch）。
var ErrNotFound = errors.New("not found")

// ErrDivideByZero 另一个哨兵错误。
var ErrDivideByZero = errors.New("divide by zero")

// ValidationError 自定义错误：struct 实现 error 接口即可。
// 对应 Java 的自定义 Exception，但可以携带结构化字段，调用方用 errors.As 取出。
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("字段 %s 校验失败: %s", e.Field, e.Reason)
}

// Validate 返回自定义错误 —— 错误就是普通返回值。
func Validate(name string) error {
	if name == "" {
		return &ValidationError{Field: "name", Reason: "不能为空"}
	}
	return nil
}

// FindUser 演示哨兵错误的包装。
func FindUser(id string) error {
	if id == "" {
		return ErrNotFound
	}
	if id == "ghost" {
		return fmt.Errorf("查询用户 %q: %w", id, ErrNotFound) // %w 包装，保留因果链
	}
	return nil
}

// LoadWrap %w 包装错误：保留因果链（对应 Java 异常的 cause 链）。
func LoadWrap(id string) error {
	if err := Validate(id); err != nil {
		return fmt.Errorf("加载用户 %q 失败: %w", id, err)
	}
	return nil
}

// InspectLoadError errors.As：沿因果链解包找到具体错误类型。
// 对应 Java 的 catch (ValidationException e)；errors.Is 则对应"是不是这个异常/原因"。
func InspectLoadError(id string) (isValidation bool, reason string) {
	err := LoadWrap(id)
	var ve *ValidationError
	if errors.As(err, &ve) {
		return true, ve.Reason
	}
	if err != nil {
		return false, err.Error()
	}
	return false, ""
}

// CheckNotFound errors.Is：判断因果链里是否包含目标哨兵错误。
func CheckNotFound(id string) bool {
	return errors.Is(FindUser(id), ErrNotFound)
}

// SafeDivide panic/recover：recover 必须在 defer 的函数里才生效。
// 哲学差异：Java 用异常做常规控制流；Go 的 error 才是常规路径，
// panic 只留给"不可能发生"的程序 bug（越界、nil 解引用、初始化失败）。
func SafeDivide(a, b int) (result int, err error) {
	defer func() {
		if r := recover(); r != nil {
			// recover() 返回 any —— panic 的值可以是任意类型。
			// 只有 error 类型才接 %w 保住因果链（errors.Is 仍可用）；其余转成普通错误信息。
			if e, ok := r.(error); ok {
				err = fmt.Errorf("recovered: %w", e)
			} else {
				err = fmt.Errorf("recovered: %v", r)
			}
		}
	}()
	if b == 0 {
		panic(ErrDivideByZero) // 演示用；生产代码应返回 error 而不是 panic
	}
	return a / b, nil
}
