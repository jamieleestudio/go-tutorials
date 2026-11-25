package kernel

import "context"

// TenantContext 租户上下文（对应 Java 版 kernel.TenantContext 的 ThreadLocal）。
//
// 关键差异：Java 用 ThreadLocal 存"当前请求的租户"；Go 的 HTTP handler 可能跑在
// 任意 goroutine 上，ThreadLocal 语义不存在 —— Go 的对应物是【context.Context value】：
// 租户 ID 随请求 ctx 一路传递，天然安全（每个请求一个 ctx，不会串）。
type tenantKey struct{}

// WithTenant 把租户 ID 放入 ctx（对应 Java 的 TenantContext.set）。
func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantKey{}, tenantID)
}

// CurrentTenant 取当前租户 ID（对应 Java 的 TenantContext.currentTenantId）。
// 未设置时返回空串（调用方决定缺省行为，如 system 域缺省 "default"）。
func CurrentTenant(ctx context.Context) string {
	if v, ok := ctx.Value(tenantKey{}).(string); ok {
		return v
	}
	return ""
}
