// erp-app-mobile 的 Go 对应物：仅 mobile 端的部署形态（端口 8082）。
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
	cfg := assemble.LoadConfig(assemble.MobileEnds, "8082")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := assemble.New(ctx, cfg)
	if err != nil {
		slog.Error("failed to start erp mobile", "error", err)
		os.Exit(1)
	}

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: app.Engine}
	go func() {
		slog.Info("erp mobile started", "port", cfg.Port, "ends", "mobile")
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
