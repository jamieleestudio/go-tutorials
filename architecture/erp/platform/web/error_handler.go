// Package web 是平台层的 Web 底座（对应 Java 版 erp-platform-web 的
// GlobalExceptionHandler + WebMvcConfig）。
package web

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/platform/kernel"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/shared"
)

// ErrorHandler 全局错误处理（对应 Java 版 @RestControllerAdvice GlobalExceptionHandler）。
//
// erp 版的错误映射与 P1/P2 不同（对齐 Java 版 erp）：
//   - shared.DomainError(ENTITY_NOT_FOUND)   → 404
//   - shared.DomainError(BUSINESS_RULE_...)  → 422（erp 用 422 表达"规则被违反"）
//   - kernel.BizException                    → 用其自带 code（校验失败 400 等）
//   - 其他                                   → 500（记录日志，不泄露内部细节）
//
// 所有响应统一 Result 包装：{"success":false,"code":"...","message":"...","data":null}
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 {
			return
		}
		err := c.Errors.Last().Err

		var domErr *shared.DomainError
		if errors.As(err, &domErr) {
			status := http.StatusUnprocessableEntity // 业务规则 → 422
			code := kernel.CodeBusinessRule
			if domErr.Kind == shared.KindEntityNotFound {
				status = http.StatusNotFound
				code = kernel.CodeNotFound
			}
			c.JSON(status, kernel.Fail(code, domErr.Message))
			c.Abort()
			return
		}

		var bizErr *kernel.BizException
		if errors.As(err, &bizErr) {
			status := http.StatusBadRequest
			switch bizErr.Code {
			case kernel.CodeUnauthorized:
				status = http.StatusUnauthorized
			case kernel.CodeNotFound:
				status = http.StatusNotFound
			case kernel.CodeBusinessRule:
				status = http.StatusUnprocessableEntity
			case kernel.CodeInternal:
				status = http.StatusInternalServerError
			}
			c.JSON(status, kernel.Fail(bizErr.Code, bizErr.Message))
			c.Abort()
			return
		}

		slog.Error("unexpected error", "error", err)
		c.JSON(http.StatusInternalServerError, kernel.Fail(kernel.CodeInternal, "An unexpected error occurred"))
		c.Abort()
	}
}

// BindJSON 统一的请求体绑定 + 校验失败处理。
// 校验失败抛 BizException(400) —— 校验在接口层完成（对应 jakarta @Valid）。
func BindJSON(c *gin.Context, obj any) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		_ = c.Error(kernel.NewBizException(kernel.CodeBadRequest, err.Error()))
		return false
	}
	return true
}
