// payment 服务入口：REST :8082 对外；/rpc/payments 供 order 服务远程调用
// （pay 按 orderId 幂等 —— 分布式重试安全的关键）。
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/jamieleestudio/go-tutorials/architecture/distributed/payment/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, app.DefaultConfig()); err != nil {
		os.Exit(1)
	}
}
