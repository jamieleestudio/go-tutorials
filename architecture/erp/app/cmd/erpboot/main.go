// erp-app-boot 的 Go 对应物：全部端 + mq + job 的部署形态（端口 8080）。
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
	cfg := assemble.LoadConfig(assemble.BootEnds, "8080")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := assemble.New(ctx, cfg)
	if err != nil {
		slog.Error("failed to start erp boot", "error", err)
		os.Exit(1)
	}

	// 仅 boot 形态启动定时任务（对应 @EnableScheduling 只出现在 ErpApplication）
	app.StartAttendanceDailyJob(ctx)

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: app.Engine}
	go func() {
		slog.Info("erp boot started", "port", cfg.Port, "ends", "web+admin+mobile+openapi+internal+device+mq+job")
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
