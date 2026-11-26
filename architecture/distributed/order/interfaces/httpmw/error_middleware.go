// Package httpmw 全局 HTTP 中间件 —— order 服务自己的 GlobalExceptionHandler。
// 微服务自治：错误契约（{errorCode, message}）每个服务自持一份。
package httpmw

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jamieleestudio/go-tutorials/architecture/distributed/shared"
)

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

func BindJSON(c *gin.Context, obj any) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		_ = c.Error(shared.NewBusinessRuleViolation("VALIDATION_ERROR", err.Error()))
		return false
	}
	return true
}
