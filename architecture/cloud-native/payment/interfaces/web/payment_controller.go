// Package web 支付上下文的 REST 接口层（对应 Java 的 PaymentController）。
package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jamieleestudio/go-tutorials/architecture/cloud-native/payment/api"
	"github.com/jamieleestudio/go-tutorials/architecture/cloud-native/payment/application"
	"github.com/jamieleestudio/go-tutorials/architecture/cloud-native/payment/interfaces/httpmw"
	"github.com/shopspring/decimal"
)

type PaymentController struct {
	paymentService application.PaymentAppService
}

func NewPaymentController(paymentService application.PaymentAppService) *PaymentController {
	return &PaymentController{paymentService: paymentService}
}

func (c *PaymentController) Register(r gin.IRouter) {
	g := r.Group("/api/payments")
	g.POST("", c.createPayment)
	g.POST("/:paymentId/process", c.processPayment)
	g.GET("", c.getAllPayments)
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

func paymentResponseFrom(dto *api.PaymentDto) PaymentResponse {
	return PaymentResponse{
		PaymentID: dto.PaymentID,
		OrderID:   dto.OrderID,
		Amount:    dto.Amount,
		Status:    dto.Status,
	}
}

func (c *PaymentController) createPayment(ctx *gin.Context) {
	var req CreatePaymentRequest
	if !httpmw.BindJSON(ctx, &req) {
		return
	}
	dto, err := c.paymentService.CreatePayment(ctx.Request.Context(), application.CreatePaymentCommand{
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
	dto, err := c.paymentService.ProcessPayment(ctx.Request.Context(), application.ProcessPaymentCommand{
		PaymentID: ctx.Param("paymentId"),
	})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusOK, paymentResponseFrom(dto))
}

func (c *PaymentController) getPayment(ctx *gin.Context) {
	dto, err := c.paymentService.GetPaymentByID(ctx.Request.Context(), application.GetPaymentByIDQuery{
		PaymentID: ctx.Param("paymentId"),
	})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusOK, paymentResponseFrom(dto))
}

func (c *PaymentController) getAllPayments(ctx *gin.Context) {
	dtos, err := c.paymentService.GetAllPayments(ctx.Request.Context(), application.GetAllPaymentsQuery{})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	responses := make([]PaymentResponse, 0, len(dtos))
	for _, dto := range dtos {
		responses = append(responses, paymentResponseFrom(dto))
	}
	ctx.JSON(http.StatusOK, responses)
}
