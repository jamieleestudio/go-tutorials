package cloudkit

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Serve 云原生运行入口：信号 → 摘流 → 排水 → 收尾。
//
// 完整的 K8s pod 终止序列（对照 deploy/k8s/*.yaml 里的注释）：
//
//  1. kubelet 发 SIGTERM（main 里 signal.NotifyContext 已把它变成 ctx 取消）；
//     同时 preStop hook 里 sleep 5s —— endpoints 还在摘除中，pod 先不退出；
//  2. 本函数收到 ctx.Done：readiness=false —— 后续探针 503，Service 摘流，
//     新请求不再路由进来；
//  3. server.Shutdown：停止接受新连接，等存量请求处理完（最多 drain 超时）；
//  4. cleanup：关闭数据库等资源；
//  5. 进程退出 → kubelet 在 terminationGracePeriodSeconds 内确认。
//
// 对照 Java 版：Spring Boot 3 有 server.shutdown: graceful，但 Java 版 P7 没有
// 配置它（也没有 preStop / terminationGracePeriodSeconds）—— 这是 Go 版的
// 已知增强点，deploy/ 的 YAML 里都补上了，注释里逐处标注。
//
// drain 是排水窗口：应 ≥ K8s readiness 传播时间，< terminationGracePeriodSeconds。
//
// ln 由调用方创建（net.Listen）并传入 —— bind 先于 ready 翻转：
// 端口开着但 readiness 503 的窗口里，K8s 探针看到的仍是"未就绪"，不会放流量进来。
func Serve(ctx context.Context, h *Health, server *http.Server, ln net.Listener, drain time.Duration, cleanup func() error) error {
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.Serve(ln)
	}()

	select {
	case err := <-serveErr:
		// 端口被占 / 装配失败等：启动期就出错，直接失败（非优雅路径）。
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
	}

	// 1) 先摘流：SIGTERM 到达的第一件事不是关端口，而是让 readiness 变 503。
	//    K8s 把 pod 从 Service endpoints 摘除需要一点时间，越早声明越少截断请求。
	h.SetReady(false)
	slog.Info("SIGTERM received: readiness OUT_OF_SERVICE, draining")

	// 2) 排水：不再接受新连接，等存量请求完成。
	shutdownCtx, cancel := context.WithTimeout(context.Background(), drain)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Warn("drain timeout or shutdown error", "error", err)
	}

	// 3) 收尾：关库等资源（在摘流之后 —— 此时已无业务请求使用它们）。
	if cleanup != nil {
		if err := cleanup(); err != nil {
			slog.Warn("cleanup error", "error", err)
		}
	}
	slog.Info("graceful shutdown complete")
	return nil
}
