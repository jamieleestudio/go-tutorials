// Package web 支付服务的 REST 接口层（对应 Java 的 PaymentController）。
package web

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jamieleestudio/go-tutorials/architecture/distributed/payment/api"
	"github.com/jamieleestudio/go-tutorials/architecture/distributed/shared"
	"github.com/shopspring/decimal"
)

// PaymentRpcController payment 服务暴露给 order 服务的内部 RPC 契约。
//
// 对应 Java 的 PaymentRpcController：把 provider 定义的 api.PaymentService
// 映射到 HTTP 端点 —— 对外 REST（/api/payments）与对内 RPC（/rpc/payments）
// 在同一进程里并存，路径即边界。
type PaymentRpcController struct {
	payments api.PaymentService
}

func NewPaymentRpcController(payments api.PaymentService) *PaymentRpcController {
	return &PaymentRpcController{payments: payments}
}

// Register 挂载 RPC 端点。
func (c *PaymentRpcController) Register(r gin.IRouter) {
	g := r.Group("/rpc/payments")
	g.POST("/pay", c.pay)
	g.POST("/:paymentId/refund", c.refund)
}

// pay 幂等支付：同一订单重复支付返回同一个支付单 ID。
// 这是 order 侧"网络层重试敢发第二次"的前提（对应 Java 同名端点）。
// 响应体是纯文本 paymentId —— 与 Java 版保持一致，不是 JSON。
func (c *PaymentRpcController) pay(ctx *gin.Context) {
	orderID := ctx.Query("orderId")
	amountRaw := ctx.Query("amount")
	if orderID == "" {
		_ = ctx.Error(shared.NewBusinessRuleViolation("VALIDATION_ERROR", "orderId is required"))
		return
	}
	amount, err := decimal.NewFromString(amountRaw)
	if err != nil {
		_ = ctx.Error(shared.NewBusinessRuleViolation("VALIDATION_ERROR", fmt.Sprintf("invalid amount %q", amountRaw)))
		return
	}
	paymentID, err := c.payments.Pay(ctx.Request.Context(), orderID, amount)
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.String(http.StatusOK, paymentID)
}

// refund 退款（200 空响应体）。
func (c *PaymentRpcController) refund(ctx *gin.Context) {
	if err := c.payments.Refund(ctx.Request.Context(), ctx.Param("paymentId")); err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.Status(http.StatusOK)
}
