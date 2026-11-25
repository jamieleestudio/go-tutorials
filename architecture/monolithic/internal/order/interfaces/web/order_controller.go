// Package web 订单上下文的 REST 接口层（gin）。
//
// 对应 Java 的 order.interfaces.web 包：只依赖应用服务接口（orderapp），
// 绝不触碰 impl / infrastructure（架构守护测试保证）。
package web

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	orderapp "github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/order/application"
	orderdomain "github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/order/domain"
	"github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/platform/httpmw"
	"github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/shared"
	"github.com/shopspring/decimal"
)

// OrderController 对应 Java 的 OrderController（@RestController → gin handler struct）。
type OrderController struct {
	orderService orderapp.OrderService
}

func NewOrderController(orderService orderapp.OrderService) *OrderController {
	return &OrderController{orderService: orderService}
}

// Register 注册路由（对应 @RequestMapping("/api/orders")）。
func (c *OrderController) Register(r gin.IRouter) {
	g := r.Group("/api/orders")
	g.POST("", c.createOrder)
	g.POST("/:orderId/pay", c.payOrder)
	g.POST("/:orderId/cancel", c.cancelOrder)
	g.GET("/:orderId", c.getOrder)
	g.GET("", c.getOrdersByCustomer)
}

// --- 请求 / 响应 DTO（接口层私有，对应 Java 的 record） ---

type CreateOrderRequest struct {
	CustomerID string             `json:"customerId" binding:"required"`
	Lines      []OrderLineRequest `json:"lines" binding:"required,min=1,dive"`
}

// OrderLineRequest binding 标签对应 Java 的 jakarta.validation 注解。
type OrderLineRequest struct {
	ProductID string `json:"productId" binding:"required"`
	Quantity  int    `json:"quantity" binding:"required,gt=0"`
}

type CancelOrderRequest struct {
	Reason string `json:"reason"`
}

type OrderLineResponse struct {
	ProductID   string          `json:"productId"`
	ProductName string          `json:"productName"`
	Quantity    int             `json:"quantity"`
	UnitPrice   decimal.Decimal `json:"unitPrice"`
}

type OrderResponse struct {
	OrderID     string              `json:"orderId"`
	CustomerID  string              `json:"customerId"`
	Status      orderdomain.OrderStatus `json:"status"`
	TotalAmount decimal.Decimal     `json:"totalAmount"`
	Lines       []OrderLineResponse `json:"lines"`
}

func orderResponseFrom(dto *orderapp.OrderDto) OrderResponse {
	lines := make([]OrderLineResponse, 0, len(dto.Lines))
	for _, l := range dto.Lines {
		lines = append(lines, OrderLineResponse{
			ProductID:   l.ProductID,
			ProductName: l.ProductName,
			Quantity:    l.Quantity,
			UnitPrice:   l.UnitPrice,
		})
	}
	return OrderResponse{
		OrderID:     dto.OrderID,
		CustomerID:  dto.CustomerID,
		Status:      dto.Status,
		TotalAmount: dto.TotalAmount,
		Lines:       lines,
	}
}

// --- Handlers ---

func (c *OrderController) createOrder(ctx *gin.Context) {
	var req CreateOrderRequest
	if !httpmw.BindJSON(ctx, &req) {
		return
	}
	lines := make([]orderapp.OrderLine, 0, len(req.Lines))
	for _, l := range req.Lines {
		lines = append(lines, orderapp.OrderLine{ProductID: l.ProductID, Quantity: l.Quantity})
	}
	dto, err := c.orderService.CreateOrder(ctx.Request.Context(), orderapp.CreateOrderCommand{
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
	if err := c.orderService.PayOrder(ctx.Request.Context(), orderapp.PayOrderCommand{
		OrderID: ctx.Param("orderId"),
	}); err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.Status(http.StatusOK)
}

// cancelOrder 请求体可选（对应 Java 的 @RequestBody(required = false)）。
func (c *OrderController) cancelOrder(ctx *gin.Context) {
	var req CancelOrderRequest
	if ctx.Request.Body != nil && ctx.Request.ContentLength != 0 {
		if !httpmw.BindJSON(ctx, &req) {
			return
		}
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "No reason"
	}
	if err := c.orderService.CancelOrder(ctx.Request.Context(), orderapp.CancelOrderCommand{
		OrderID: ctx.Param("orderId"),
		Reason:  reason,
	}); err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.Status(http.StatusOK)
}

func (c *OrderController) getOrder(ctx *gin.Context) {
	dto, err := c.orderService.GetOrderByID(ctx.Request.Context(), orderapp.GetOrderByIDQuery{
		OrderID: ctx.Param("orderId"),
	})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusOK, orderResponseFrom(dto))
}

func (c *OrderController) getOrdersByCustomer(ctx *gin.Context) {
	customerID := ctx.Query("customerId")
	if customerID == "" {
		_ = ctx.Error(shared.NewBusinessRuleViolation("VALIDATION_ERROR", "customerId is required"))
		return
	}
	dtos, err := c.orderService.GetOrdersByCustomer(ctx.Request.Context(), orderapp.GetOrdersByCustomerQuery{
		CustomerID: customerID,
	})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	responses := make([]OrderResponse, 0, len(dtos))
	for i := range dtos {
		responses = append(responses, orderResponseFrom(&dtos[i]))
	}
	ctx.JSON(http.StatusOK, responses)
}
