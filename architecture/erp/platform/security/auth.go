package security

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/platform/kernel"
)

// AppIDAttribute ctx 键：ApiAuthFilter 解析出的应用 ID（对应 Java 的 APP_ID_ATTRIBUTE）。
type appIDKey struct{}

// CtxAppID 从 ctx 取 ApiAuthFilter 存入的应用 ID。
func CtxAppID(c *gin.Context) string {
	if v, ok := c.Get(appIDKey{}); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// ApiAuthFilter 解析 Authorization: Bearer <token>（对应 Java 版 ApiAuthFilter）。
//
// 与 Java 一致：只做"解析并存起来"，不校验 —— 校验是 AuthMiddleware 的事。
func ApiAuthFilter() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if strings.HasPrefix(token, "Bearer ") {
			c.Set(appIDKey{}, strings.TrimPrefix(token, "Bearer "))
		}
		c.Next()
	}
}

// AuthMiddleware 校验 X-App-Id 请求头（对应 Java 版 AuthInterceptor）。
//
// 与 Java 一致：缺头 → 401 Result 格式响应。Java 版靠 WebMvcConfig 把拦截器
// addPathPatterns("/openapi/**") 限定路径；Go 版直接在本中间件里判断路径前缀。
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, "/openapi/") {
			c.Next()
			return
		}
		if strings.TrimSpace(c.GetHeader("X-App-Id")) == "" {
			c.Header("Content-Type", "application/json; charset=utf-8")
			c.AbortWithStatusJSON(http.StatusUnauthorized, kernel.Fail(kernel.CodeUnauthorized, "missing X-App-Id"))
			return
		}
		c.Next()
	}
}
