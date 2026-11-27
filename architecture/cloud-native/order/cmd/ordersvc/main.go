// order 服务入口：REST :8080 对外（K8s 统一端口）；跨服务调用是 HTTP RPC 客户端，
// 基址默认 K8s Service DNS（http://payment-service:8080 / http://product-service:8080），
// PAYMENT_SERVICE_URL / PRODUCT_SERVICE_URL 环境变量可覆盖。
// SIGTERM → 优雅停机：摘流 → 排水 → 关库（见 shared/cloudkit）。
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/jamieleestudio/go-tutorials/architecture/cloud-native/order/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, app.DefaultConfig()); err != nil {
		os.Exit(1)
	}
}
