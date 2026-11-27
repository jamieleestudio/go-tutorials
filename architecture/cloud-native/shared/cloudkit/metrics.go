package cloudkit

import (
	"fmt"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// Metrics 极简指标集：手写 Prometheus 文本格式，零依赖 ——
// 对应 Java actuator 的 /actuator/metrics（Java 走 micrometer 传递依赖）。
//
// 教学边界：这里只做 counter + gauge 的最小可用集，够 K8s/HPA 观测；
// 生产项目直接用 prometheus/client_golang（histogram/摘要/注册表都齐）。
type Metrics struct {
	health *Health

	mu       sync.Mutex               // 保护 counters 的创建（读路径无锁：atomic）
	counters map[string]*atomic.Int64 // key 已含完整 label 集，如 http_requests_total{method="GET",code="200"}
}

// NewMetrics 构造指标集（挂同一个 Health，共享进程启动时刻）。
func NewMetrics(h *Health) *Metrics {
	return &Metrics{health: h, counters: map[string]*atomic.Int64{}}
}

// IncCounter 原子递增一个计数器（key 需自带 label 集）。
func (m *Metrics) IncCounter(key string) {
	m.mu.Lock()
	c, ok := m.counters[key]
	if !ok {
		c = &atomic.Int64{}
		m.counters[key] = c
	}
	m.mu.Unlock()
	c.Add(1)
}

// Middleware HTTP 指标中间件：按 method+code 计数 http_requests_total。
//
// 教学点：label 刻意【不含路径】—— REST 路径带资源 ID（/api/orders/{uuid}），
// 直接做 label 会造成基数爆炸（Prometheus 的经典事故）；
// 生产项目用路由模板（gin 的 c.FullPath()）或 RED 方法库解决。
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 包装 ResponseWriter 以在 handler 执行后拿到状态码。
		rec := &statusRecorder{ResponseWriter: w, code: http.StatusOK}
		next.ServeHTTP(rec, r)
		m.IncCounter(fmt.Sprintf("http_requests_total{method=%q,code=%q}", r.Method, codeClass(rec.code)))
	})
}

// statusRecorder 记录状态码的 ResponseWriter 包装。
type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.code = code
	r.ResponseWriter.WriteHeader(code)
}

// codeClass 保留三位状态码（教学版直接用全码；也可折叠成 2xx/4xx/5xx）。
func codeClass(code int) string { return fmt.Sprintf("%d", code) }

// Handler GET /metrics —— Prometheus 文本格式（exposition format）。
// 内容：业务计数器 + 进程级 gauge（goroutine 数 / 堆内存 / 运行时长）。
func (m *Metrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		var b strings.Builder

		// 业务计数器（按 key 排序：输出稳定，便于测试与 diff）
		m.mu.Lock()
		keys := make([]string, 0, len(m.counters))
		for k := range m.counters {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		values := make([]int64, 0, len(keys))
		for _, k := range keys {
			values = append(values, m.counters[k].Load())
		}
		m.mu.Unlock()

		if len(keys) > 0 {
			b.WriteString("# HELP http_requests_total Total HTTP requests processed.\n")
			b.WriteString("# TYPE http_requests_total counter\n")
			for i, k := range keys {
				// key 形如 http_requests_total{...}；裸计数器也允许（无 label）
				b.WriteString(k)
				b.WriteString(fmt.Sprintf(" %d\n", values[i]))
			}
		}

		// 进程级 gauge
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		fmt.Fprintf(&b, "# HELP go_goroutines Number of goroutines that currently exist.\n")
		fmt.Fprintf(&b, "# TYPE go_goroutines gauge\n")
		fmt.Fprintf(&b, "go_goroutines %d\n", runtime.NumGoroutine())
		fmt.Fprintf(&b, "# HELP go_memstats_alloc_bytes Number of bytes allocated and still in use.\n")
		fmt.Fprintf(&b, "# TYPE go_memstats_alloc_bytes gauge\n")
		fmt.Fprintf(&b, "go_memstats_alloc_bytes %d\n", ms.Alloc)
		fmt.Fprintf(&b, "# HELP process_uptime_seconds Seconds since process start.\n")
		fmt.Fprintf(&b, "# TYPE process_uptime_seconds gauge\n")
		fmt.Fprintf(&b, "process_uptime_seconds %.2f\n", m.health.Uptime().Seconds())

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = w.Write([]byte(b.String()))
	}
}
