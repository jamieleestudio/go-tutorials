// payment 服务入口：REST :8082（对外）+ gRPC :9182（对内，供 order 调用）。
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/payment/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, app.DefaultConfig()); err != nil {
		os.Exit(1)
	}
}
