// Package web 订单上下文的 REST 接口层（对应 Java 的 OrderController）。
package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jamieleestudio/go-tutorials/architecture/distributed/order/application"
	"github.com/jamieleestudio/go-tutorials/architecture/distributed/order/interfaces/httpmw"
)

type OrderController struct {
	orderService application.OrderService
}

func NewOrderController(orderService application.OrderService) *OrderController {
	return &OrderController{orderService: orderService}
}

func (c *OrderController) Register(r gin.IRouter) {
	g := r.Group("/api/orders")
	g.POST("", c.createOrder)
	g.POST("/:orderId/pay", c.payOrder)
	g.POST("/:orderId/cancel", c.cancelOrder)
	g.GET("", c.getOrdersByCustomer)
	g.GET("/:orderId", c.getOrder)
}

type CreateOrderRequest struct {
	CustomerID string             `json:"customerId" binding:"required"`
	Lines      []OrderLineRequest `json:"lines" binding:"required,min=1"`
}

type OrderLineRequest struct {
	ProductID string `json:"productId" binding:"required"`
	Quantity  int    `json:"quantity" binding:"required,gt=0"`
}

type CancelOrderRequest struct {
	Reason string `json:"reason"`
}

type OrderResponse struct {
	OrderID     string              `json:"orderId"`
	CustomerID  string              `json:"customerId"`
	Status      string              `json:"status"`
	TotalAmount string              `json:"totalAmount"`
	Lines       []OrderLineResponse `json:"lines"`
}

type OrderLineResponse struct {
	ProductID   string `json:"productId"`
	ProductName string `json:"productName"`
	Quantity    int    `json:"quantity"`
	UnitPrice   string `json:"unitPrice"`
}

func orderResponseFrom(dto *application.OrderDto) OrderResponse {
	lines := make([]OrderLineResponse, 0, len(dto.Lines))
	for _, line := range dto.Lines {
		lines = append(lines, OrderLineResponse{
			ProductID:   line.ProductID,
			ProductName: line.ProductName,
			Quantity:    line.Quantity,
			UnitPrice:   line.UnitPrice.StringFixed(2),
		})
	}
	return OrderResponse{
		OrderID:     dto.OrderID,
		CustomerID:  dto.CustomerID,
		Status:      string(dto.Status),
		TotalAmount: dto.TotalAmount.StringFixed(2),
		Lines:       lines,
	}
}

func (c *OrderController) createOrder(ctx *gin.Context) {
	var req CreateOrderRequest
	if !httpmw.BindJSON(ctx, &req) {
		return
	}
	lines := make([]application.OrderLine, 0, len(req.Lines))
	for _, line := range req.Lines {
		lines = append(lines, application.OrderLine{ProductID: line.ProductID, Quantity: line.Quantity})
	}
	dto, err := c.orderService.CreateOrder(ctx.Request.Context(), application.CreateOrderCommand{
		CustomerID: req.CustomerID,
		Lines:      lines,
	})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusCreated, orderResponseFrom(dto))
}

func (c *OrderController) payOrder(ctx *gin.Context) {
	if err := c.orderService.PayOrder(ctx.Request.Context(), application.PayOrderCommand{
		OrderID: ctx.Param("orderId"),
	}); err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "ok"})
}

func (c *OrderController) cancelOrder(ctx *gin.Context) {
	// 与 Java 一致：请求体可选（required=false），reason 缺省 "No reason"
	var req CancelOrderRequest
	reason := "No reason"
	if err := ctx.ShouldBindJSON(&req); err == nil && req.Reason != "" {
		reason = req.Reason
	}
	if err := c.orderService.CancelOrder(ctx.Request.Context(), application.CancelOrderCommand{
		OrderID: ctx.Param("orderId"),
		Reason:  reason,
	}); err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "ok"})
}

func (c *OrderController) getOrder(ctx *gin.Context) {
	dto, err := c.orderService.GetOrderByID(ctx.Request.Context(), application.GetOrderByIDQuery{
		OrderID: ctx.Param("orderId"),
	})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusOK, orderResponseFrom(dto))
}

func (c *OrderController) getOrdersByCustomer(ctx *gin.Context) {
	dtos, err := c.orderService.GetOrdersByCustomer(ctx.Request.Context(), application.GetOrdersByCustomerQuery{
		CustomerID: ctx.Query("customerId"),
	})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	responses := make([]OrderResponse, 0, len(dtos))
	for _, dto := range dtos {
		responses = append(responses, orderResponseFrom(dto))
	}
	ctx.JSON(http.StatusOK, responses)
}
