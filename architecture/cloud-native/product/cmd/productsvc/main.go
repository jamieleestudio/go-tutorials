// product 服务入口：REST :8080 对外（K8s 统一端口）；/rpc/products 供 order 服务远程调用。
// SIGTERM → 优雅停机：摘流 → 排水 → 关库（见 shared/cloudkit）。
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/jamieleestudio/go-tutorials/architecture/cloud-native/product/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, app.DefaultConfig()); err != nil {
		os.Exit(1)
	}
}
