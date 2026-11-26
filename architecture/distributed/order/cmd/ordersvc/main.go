// order 服务入口：REST :8081 对外；跨服务的 HTTP RPC 客户端指向
// payment :8082 / product :8083（可用 PAYMENT_SERVICE_URL / PRODUCT_SERVICE_URL 覆盖）。
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/jamieleestudio/go-tutorials/architecture/distributed/order/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, app.DefaultConfig()); err != nil {
		os.Exit(1)
	}
}
