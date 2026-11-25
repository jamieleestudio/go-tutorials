// erp-app-admin 的 Go 对应物：web + admin 端的部署形态（端口 8081）。
// Java 版靠 excludeFilters 正则排除其他端；Go 版靠 EndSet 不注册。
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

	"github.com/jamieleestudio/go-tutorials/architecture/erp/app/assemble"
)

func main() {
	cfg := assemble.LoadConfig(assemble.AdminEnds, "8081")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := assemble.New(ctx, cfg)
	if err != nil {
		slog.Error("failed to start erp admin", "error", err)
		os.Exit(1)
	}

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: app.Engine}
	go func() {
		slog.Info("erp admin started", "port", cfg.Port, "ends", "web+admin")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
	}
	if sqlDB, err := app.DB.DB(); err == nil {
		_ = sqlDB.Close()
	}
	slog.Info("bye")
}
