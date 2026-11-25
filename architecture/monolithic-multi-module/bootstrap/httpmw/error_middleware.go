// Package httpmw 全局 HTTP 中间件，对应 Java 版 bootstrap 模块的 GlobalExceptionHandler。
//
// 多模块版的差异：P1 中它属于 monolithic 的 platform 包；P2 里它是 bootstrap（部署单元）
// 的职责 —— Java 版也把 @RestControllerAdvice 放在了 bootstrap 模块，语义一致。
package httpmw

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/shared"
)

// ErrorHandler 把错误统一渲染为 HTTP 响应。
//
// handler 里出错误时调用 c.Error(err) 后 return，
// 本中间件在 c.Next() 之后统一映射：
//   - shared.DomainError(ENTITY_NOT_FOUND)  → 404 {"errorCode", "message"}
//   - shared.DomainError(BUSINESS_RULE_...) → 400
//   - 其他                                   → 500（记录日志，不泄露内部细节）
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
// 校验失败时转成 VALIDATION_ERROR 业务错误（HTTP 400）。
func BindJSON(c *gin.Context, obj any) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		_ = c.Error(shared.NewBusinessRuleViolation("VALIDATION_ERROR", err.Error()))
		return false
	}
	return true
}
