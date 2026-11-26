// Package httpmw 全局 HTTP 中间件 —— product 服务自己的 GlobalExceptionHandler。
//
// 对应 Java 版每个微服务各自的 @RestControllerAdvice：
// 微服务自治，错误契约（{errorCode, message}）每个服务自持。
//   - shared.DomainError(ENTITY_NOT_FOUND)      → 404 {"errorCode", "message"}
//   - shared.DomainError(BUSINESS_RULE_VIOLATION) → 400
//   - 其他                                        → 500（记录日志，不泄露内部细节）
package httpmw

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jamieleestudio/go-tutorials/architecture/microservices/shared"
)

// ErrorHandler 把错误统一渲染为 HTTP 响应。
// handler 里出错误时调用 c.Error(err) 后 return，本中间件统一映射。
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 {
			return
		}
		err := c.Errors.Last().Err

		var domErr *shared.DomainError
		if errors.As(err, &domErr) {
			status := http.StatusBadRequest
			if domErr.Kind == shared.KindEntityNotFound {
				status = http.StatusNotFound
			}
			c.JSON(status, gin.H{"errorCode": domErr.Code, "message": domErr.Message})
			c.Abort()
			return
		}

		slog.Error("unexpected error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"errorCode": "INTERNAL_ERROR",
			"message":   "An unexpected error occurred",
		})
		c.Abort()
	}
}

// BindJSON 统一的请求体绑定 + 校验失败处理。
// 校验失败时转为 VALIDATION_ERROR 业务错误（HTTP 400）。
func BindJSON(c *gin.Context, obj any) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		_ = c.Error(shared.NewBusinessRuleViolation("VALIDATION_ERROR", err.Error()))
		return false
	}
	return true
}
