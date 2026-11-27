package cloudkit

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// direct 无代理 HTTP 客户端：127.0.0.1 的探针请求绝不能被系统代理截走
// （代理会"成功建立连接"然后无限等待，DefaultClient 又没有超时 —— 本地冒烟/
// 单测的经典坑）。
var direct = &http.Client{Transport: &http.Transport{Proxy: nil}}

func getBody(t *testing.T, url string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := direct.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if err != nil {
		t.Fatalf("read %s: %v", url, err)
	}
	return resp.StatusCode, string(raw)
}

func TestLivenessAlwaysUp(t *testing.T) {
	h := NewHealth("unit")
	// 关键语义：即使【未就绪】，liveness 也是 UP —— 依赖没就绪不该触发重启。
	if code, body := getBody(t, serveOne(t, h.LivenessHandler())); code != http.StatusOK || !strings.Contains(body, `"UP"`) {
		t.Fatalf("liveness: %d %s, want 200 UP even when not ready", code, body)
	}
}

func TestReadinessGate(t *testing.T) {
	h := NewHealth("unit")
	probe := serveOne(t, h.ReadinessHandler())

	// 初始未就绪（对应 pod 启动期）：503 OUT_OF_SERVICE → K8s 摘流不重启。
	if code, body := getBody(t, probe); code != http.StatusServiceUnavailable || !strings.Contains(body, "OUT_OF_SERVICE") {
		t.Fatalf("initial readiness: %d %s, want 503 OUT_OF_SERVICE", code, body)
	}

	h.SetReady(true)
	if code, body := getBody(t, probe); code != http.StatusOK || !strings.Contains(body, `"UP"`) {
		t.Fatalf("ready: %d %s, want 200 UP", code, body)
	}

	// SIGTERM 摘流路径：SetReady(false) → 503（pod 仍在运行，只是不接新流量）。
	h.SetReady(false)
	if code, _ := getBody(t, probe); code != http.StatusServiceUnavailable {
		t.Fatalf("after SIGTERM flip: %d, want 503", code)
	}
}

func TestAggregateAndInfo(t *testing.T) {
	t.Setenv("SERVICE_VERSION", "9.9.9-test")
	h := NewHealth("product")
	h.SetReady(true)

	if code, body := getBody(t, serveOne(t, h.AggregateHandler())); code != http.StatusOK ||
		!strings.Contains(body, `"ready":true`) {
		t.Fatalf("aggregate: %d %s", code, body)
	}

	code, body := getBody(t, serveOne(t, h.InfoHandler()))
	if code != http.StatusOK ||
		!strings.Contains(body, `"service":"product"`) ||
		!strings.Contains(body, `"version":"9.9.9-test"`) ||
		!strings.Contains(body, `"goVersion":"go`) {
		t.Fatalf("info: %d %s", code, body)
	}
}

func TestMiddlewareAndMetrics(t *testing.T) {
	h := NewHealth("unit")
	m := NewMetrics(h)

	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	base := serveOne(t, handler)

	for range 2 {
		if code, _ := getBody(t, base); code != http.StatusCreated {
			t.Fatalf("wrapped handler: %d", code)
		}
	}

	_, metrics := getBody(t, serveOne(t, m.Handler()))
	if !strings.Contains(metrics, `http_requests_total{method="GET",code="201"} 2`) {
		t.Fatalf("metrics missing counter: %s", metrics)
	}
	for _, want := range []string{"go_goroutines", "go_memstats_alloc_bytes", "process_uptime_seconds"} {
		if !strings.Contains(metrics, want) {
			t.Fatalf("metrics missing %s:\n%s", want, metrics)
		}
	}
}

// TestServeGracefulShutdownSequence 端到端验证优雅停机序列：
// 正常服务（readiness UP）→ cancel ctx（模拟 SIGTERM）→ 摘流 → 排水 → cleanup。
func TestServeGracefulShutdownSequence(t *testing.T) {
	h := NewHealth("order")
	h.SetReady(true) // 装配完成（Run 里在 Serve 前设置）

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeStatus(w, http.StatusOK, "UP")
	})}

	// cleanup 在【摘流之后】执行 —— 此时不应再有业务请求，关库才安全。
	var cleanupSawReady bool
	cleanupCalled := false
	cleanup := func() error {
		cleanupCalled = true
		cleanupSawReady = h.Ready()
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	serveDone := make(chan error, 1)
	go func() { serveDone <- Serve(ctx, h, server, lis, 3*time.Second, cleanup) }()

	base := "http://" + lis.Addr().String()
	waitFor(t, base+"/healthz", `UP`)

	cancel() // 模拟 SIGTERM
	select {
	case err := <-serveDone:
		if err != nil {
			t.Fatalf("Serve returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after cancel")
	}

	if !cleanupCalled {
		t.Fatal("cleanup not called")
	}
	if cleanupSawReady {
		t.Fatal("cleanup ran before readiness flip (drain order broken)")
	}
	if h.Ready() {
		t.Fatal("readiness should be false after graceful shutdown")
	}
	// server.Shutdown 后监听已关闭：再请求必须失败（无代理直连）。
	req, err := http.NewRequest(http.MethodGet, base+"/healthz", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if _, err := direct.Do(req); err == nil {
		t.Fatal("listener should be closed after shutdown")
	}
}

// serveOne 起一个随机端口的临时服务器，返回其 base URL（探针 handler 单测用）。
func serveOne(t *testing.T, h http.Handler) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := &http.Server{Handler: h}
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	})
	return "http://" + lis.Addr().String()
}

// waitFor 轮询直到响应体包含 want（探针/异步场景的最小工具）。
func waitFor(t *testing.T, url, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, body := getBody(t, url); strings.Contains(body, want) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s never contained %q", url, want)
}

func TestEnvOr(t *testing.T) {
	t.Setenv("CK_TEST_VAR", "from-env")
	if got := EnvOr("CK_TEST_VAR", "fallback"); got != "from-env" {
		t.Fatalf("EnvOr: %q", got)
	}
	if got := EnvOr("CK_TEST_MISSING", "fallback"); got != "fallback" {
		t.Fatalf("EnvOr fallback: %q", got)
	}
	if got := AddrFromPort("8080"); got != ":8080" {
		t.Fatalf("AddrFromPort default: %q", got)
	}
	t.Setenv("PORT", "8081")
	if got := AddrFromPort("8080"); got != ":8081" {
		t.Fatalf("AddrFromPort override: %q", got)
	}
}
