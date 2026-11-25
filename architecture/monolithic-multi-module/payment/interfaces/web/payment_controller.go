// Package web 支付上下文的 REST 接口层（对应 Java 的 PaymentController）。
package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
	paymentapp "github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/payment/application"
	"github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/shared"
	"github.com/shopspring/decimal"
)

type PaymentController struct {
	paymentService paymentapp.PaymentService
}

func NewPaymentController(paymentService paymentapp.PaymentService) *PaymentController {
	return &PaymentController{paymentService: paymentService}
}

func (c *PaymentController) Register(r gin.IRouter) {
	g := r.Group("/api/payments")
	g.POST("", c.createPayment)
	g.GET("", c.getAllPayments)
	g.POST("/:paymentId/process", c.processPayment)
	g.GET("/:paymentId", c.getPayment)
}

type CreatePaymentRequest struct {
	OrderID string          `json:"orderId" binding:"required"`
	Amount  decimal.Decimal `json:"amount" binding:"required"`
}

type PaymentResponse struct {
	PaymentID string          `json:"paymentId"`
	OrderID   string          `json:"orderId"`
	Amount    decimal.Decimal `json:"amount"`
	Status    string          `json:"status"`
}

func paymentResponseFrom(dto *paymentapp.PaymentDto) PaymentResponse {
	return PaymentResponse{
		PaymentID: dto.PaymentID,
		OrderID:   dto.OrderID,
		Amount:    dto.Amount,
		Status:    dto.Status,
	}
}

func (c *PaymentController) createPayment(ctx *gin.Context) {
	var req CreatePaymentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		_ = ctx.Error(shared.NewBusinessRuleViolation("VALIDATION_ERROR", err.Error()))
		return
	}
	dto, err := c.paymentService.CreatePayment(ctx.Request.Context(), paymentapp.CreatePaymentCommand{
		OrderID: req.OrderID,
		Amount:  req.Amount,
	})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusOK, paymentResponseFrom(dto))
}

func (c *PaymentController) processPayment(ctx *gin.Context) {
	dto, err := c.paymentService.ProcessPayment(ctx.Request.Context(), paymentapp.ProcessPaymentCommand{
		PaymentID: ctx.Param("paymentId"),
	})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusOK, paymentResponseFrom(dto))
}

func (c *PaymentController) getPayment(ctx *gin.Context) {
	dto, err := c.paymentService.GetPaymentByID(ctx.Request.Context(), paymentapp.GetPaymentByIDQuery{
		PaymentID: ctx.Param("paymentId"),
	})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusOK, paymentResponseFrom(dto))
}

func (c *PaymentController) getAllPayments(ctx *gin.Context) {
	dtos, err := c.paymentService.GetAllPayments(ctx.Request.Context(), paymentapp.GetAllPaymentsQuery{})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	responses := make([]PaymentResponse, 0, len(dtos))
	for i := range dtos {
		responses = append(responses, paymentResponseFrom(&dtos[i]))
	}
	ctx.JSON(http.StatusOK, responses)
}
