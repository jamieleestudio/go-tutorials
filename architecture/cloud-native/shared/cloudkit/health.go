// Package cloudkit 云原生运行时设施：健康探针、就绪门、指标与优雅停机。
//
// 对应 Java 版的 spring-boot-starter-actuator + application.yml 声明式配置 ——
// Java 那边零代码：management.endpoint.health.probes.enabled: true 就有
// /actuator/health/liveness 与 /actuator/health/readiness；Go 没有等价的开箱组件，
// 本包就是它的"手写对应物"，语义逐条对齐 Spring Boot Actuator 与 K8s 探针准则：
//
//	liveness（存活）：失败 → kubelet 重启容器。只回答"进程还活着吗"，
//	                  不检查依赖 —— 外部依赖挂了重启也救不了，不该重启。
//	readiness（就绪）：失败 → 从 Service endpoints 摘流（不再接新请求），但不重启。
//	优雅停机：SIGTERM → 先摘流（readiness=false）→ 排水存量请求 → 收尾（关库）。
//
// 本包只用标准库（net/http + sync/atomic），不引入 gin —— 业务服务的
// gin 路由用 gin.WrapF 挂载这里的 handler（见各服务 app.go 的 Run）。
package cloudkit

import (
	"encoding/json"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"
)

// Health 进程内健康状态（对应 Spring 的 ApplicationAvailability）。
// ready 用 atomic.Bool：HTTP 探针 goroutine 与信号处理 goroutine 并发读写。
type Health struct {
	service string // 服务名（对应 Java spring.application.name）
	version string // 构建版本（SERVICE_VERSION 环境变量，12-Factor 外置配置）
	start   time.Time
	ready   atomic.Bool
}

// NewHealth 构造健康门：初始【未就绪】—— 启动早期（迁移/装配未完成）
// readinessProbe 必须失败，K8s 才不会把没准备好的 pod 放进 Service endpoints。
func NewHealth(service string) *Health {
	return &Health{
		service: service,
		version: EnvOr("SERVICE_VERSION", "dev"),
		start:   time.Now(),
	}
}

// SetReady 翻转就绪门：启动完成 → true；收到 SIGTERM → false（摘流）。
func (h *Health) SetReady(v bool) { h.ready.Store(v) }

// Ready 当前是否就绪（探针与优雅停机共用同一状态源）。
func (h *Health) Ready() bool { return h.ready.Load() }

// ServiceName 服务名。
func (h *Health) ServiceName() string { return h.service }

// statusBody Actuator 的健康响应体形状：{"status":"UP"}。
type statusBody struct {
	Status string `json:"status"`
}

func writeStatus(w http.ResponseWriter, code int, status string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(statusBody{Status: status})
}

// LivenessHandler GET /healthz/liveness —— 对应 Java 的 /actuator/health/liveness。
//
// 语义（K8s 探针设计准则）：只要进程还能应答就是 UP，【不检查数据库等依赖】——
// 依赖故障不该触发重启，重启解决不了外部依赖的问题。
// 失败（DOWN）→ kubelet 重启容器；成功 → 维持现状。
func (h *Health) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeStatus(w, http.StatusOK, "UP") // 进程能应答即为活
	}
}

// ReadinessHandler GET /healthz/readiness —— 对应 Java 的 /actuator/health/readiness。
//
// 语义：未就绪（启动中 / 正在优雅停机）→ 503 {"status":"OUT_OF_SERVICE"}
// （Spring Actuator 的措辞），K8s 摘流不重启；就绪 → 200 {"status":"UP"}。
// 这是"重启决策（liveness）"与"摘流决策（readiness）"分离的核心。
func (h *Health) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !h.ready.Load() {
			writeStatus(w, http.StatusServiceUnavailable, "OUT_OF_SERVICE")
			return
		}
		writeStatus(w, http.StatusOK, "UP")
	}
}

// AggregateHandler GET /healthz —— 对应 Java 的 /actuator/health 总端点
// （聚合视图：服务身份 + 存活状态 + 就绪状态）。
func (h *Health) AggregateHandler() http.HandlerFunc {
	type aggregate struct {
		Status string `json:"status"`
		Ready  bool   `json:"ready"`
	}
	return func(w http.ResponseWriter, _ *http.Request) {
		status := "UP"
		if !h.ready.Load() {
			status = "OUT_OF_SERVICE"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(aggregate{Status: status, Ready: h.ready.Load()})
	}
}

// infoBody /info 响应体（对应 Spring 的 /actuator/info：构建信息）。
type infoBody struct {
	Service       string  `json:"service"`
	Version       string  `json:"version"`
	GoVersion     string  `json:"goVersion"`
	UptimeSeconds float64 `json:"uptimeSeconds"`
}

// InfoHandler GET /info —— 构建与运行信息（12-Factor：版本来自环境变量）。
func (h *Health) InfoHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(infoBody{
			Service:       h.service,
			Version:       h.version,
			GoVersion:     runtime.Version(),
			UptimeSeconds: h.Uptime().Seconds(),
		})
	}
}

// Uptime 进程运行时长（指标端点也用）。
func (h *Health) Uptime() time.Duration { return time.Since(h.start) }
