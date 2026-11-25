// MonolithicApplication 的 Go 对应物：应用入口 + 优雅停机。
//
// Java 版 main 只有 SpringApplication.run 一行，生命周期由容器管理；
// Go 需要自己管理 HTTP server 的启停 —— 这是学习点之一。
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/app"
	"github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/platform/config"
)

func main() {
	cfg := config.Load()

	application, err := app.New(context.Background(), cfg)
	if err != nil {
		slog.Error("failed to start monolithic app", "error", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: application.Engine,
	}

	// 异步启动 HTTP 服务
	go func() {
		slog.Info("monolithic app started", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server error", "error", err)
			os.Exit(1)
		}
	}()

	// 等待中断信号（Ctrl+C / SIGTERM），优雅停机
	notifyCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-notifyCtx.Done()
	slog.Info("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
	}
	if sqlDB, err := application.DB.DB(); err == nil {
		_ = sqlDB.Close()
	}
	slog.Info("bye")
}
