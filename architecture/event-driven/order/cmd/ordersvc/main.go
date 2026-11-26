// order 服务入口：REST :8081（对外）。
// 不监听 gRPC —— order 是 RPC 消费方，product 才是提供方；
// Saga 的参与经 Outbox Relay（webhook 投递）+ /internal/events 事件入口。
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/order/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, app.DefaultConfig()); err != nil {
		os.Exit(1)
	}
}
