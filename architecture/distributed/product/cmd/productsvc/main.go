// product 服务入口：REST :8083 对外；/rpc/products 供 order 服务远程调用。
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/jamieleestudio/go-tutorials/architecture/distributed/product/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, app.DefaultConfig()); err != nil {
		os.Exit(1)
	}
}
