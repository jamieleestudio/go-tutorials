package cloudkit

import "net/http"

// Route 运行时端点的注册描述 —— 用纯 net/http 表达，业务服务侧再
// gin.WrapF 挂进 gin 引擎（这样 shared 模块不必依赖 gin）。
type Route struct {
	Method  string
	Path    string
	Handler http.HandlerFunc
}

// Routes 云原生运行时端点集（每个服务都挂同一组，对应 Java 三份 application.yml
// 里逐字相同的 management 配置段）：
//
//	GET /healthz/liveness  → K8s livenessProbe（重启决策；对应 /actuator/health/liveness）
//	GET /healthz/readiness → K8s readinessProbe（摘流决策；对应 /actuator/health/readiness）
//	GET /healthz           → 聚合视图（对应 /actuator/health）
//	GET /info              → 构建/运行信息（对应 /actuator/info）
//	GET /metrics           → Prometheus 文本指标（对应 /actuator/metrics）
func Routes(h *Health, m *Metrics) []Route {
	return []Route{
		{http.MethodGet, "/healthz/liveness", h.LivenessHandler()},
		{http.MethodGet, "/healthz/readiness", h.ReadinessHandler()},
		{http.MethodGet, "/healthz", h.AggregateHandler()},
		{http.MethodGet, "/info", h.InfoHandler()},
		{http.MethodGet, "/metrics", m.Handler()},
	}
}
