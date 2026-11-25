package kernel

import "fmt"

// ErrorCode 错误码常量（对应 Java 版 kernel.ErrorCode）。
// 与 Java 版一致：错误码就是 HTTP 状态码字符串。
const (
	CodeBadRequest   = "400"
	CodeUnauthorized = "401"
	CodeNotFound     = "404"
	CodeBusinessRule = "422"
	CodeInternal     = "500"
)

// BizException 平台层业务异常（对应 Java 版 kernel.BizException）。
//
// 与 shared.DomainError 的分工：DomainError 是领域层抛出的业务规则错误（无 code），
// BizException 是接口层/应用层直接携带错误码的异常（如校验失败 400、鉴权失败 401）。
type BizException struct {
	Code    string
	Message string
}

func (e *BizException) Error() string { return fmt.Sprintf("[%s] %s", e.Code, e.Message) }

// NewBizException 构造平台业务异常。
func NewBizException(code, message string) *BizException {
	return &BizException{Code: code, Message: message}
}
