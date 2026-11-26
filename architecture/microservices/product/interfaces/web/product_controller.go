// Package web 产品上下文的 REST 接口层（对应 Java 的 ProductController）。
package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jamieleestudio/go-tutorials/architecture/microservices/product/api"
	"github.com/jamieleestudio/go-tutorials/architecture/microservices/product/application"
	"github.com/jamieleestudio/go-tutorials/architecture/microservices/product/interfaces/httpmw"
	"github.com/shopspring/decimal"
)

type ProductController struct {
	productService application.ProductAppService
}

func NewProductController(productService application.ProductAppService) *ProductController {
	return &ProductController{productService: productService}
}

func (c *ProductController) Register(r gin.IRouter) {
	g := r.Group("/api/products")
	g.POST("", c.createProduct)
	g.GET("", c.getAllProducts)
	g.GET("/:productId", c.getProduct)
}

type CreateProductRequest struct {
	Name  string          `json:"name" binding:"required"`
	Price decimal.Decimal `json:"price" binding:"required"`
	Stock int             `json:"stock" binding:"gte=0"`
}

type ProductResponse struct {
	ProductID string          `json:"productId"`
	Name      string          `json:"name"`
	Price     decimal.Decimal `json:"price"`
	Stock     int             `json:"stock"`
}

func productResponseFrom(dto *api.ProductDto) ProductResponse {
	return ProductResponse{
		ProductID: dto.ProductID,
		Name:      dto.Name,
		Price:     dto.Price,
		Stock:     dto.Stock,
	}
}

func (c *ProductController) createProduct(ctx *gin.Context) {
	var req CreateProductRequest
	if !httpmw.BindJSON(ctx, &req) {
		return
	}
	dto, err := c.productService.CreateProduct(ctx.Request.Context(), application.CreateProductCommand{
		Name:  req.Name,
		Price: req.Price,
		Stock: req.Stock,
	})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusCreated, productResponseFrom(dto))
}

func (c *ProductController) getProduct(ctx *gin.Context) {
	dto, err := c.productService.GetProductByID(ctx.Request.Context(), application.GetProductByIDQuery{
		ProductID: ctx.Param("productId"),
	})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusOK, productResponseFrom(dto))
}

func (c *ProductController) getAllProducts(ctx *gin.Context) {
	dtos, err := c.productService.GetAllProducts(ctx.Request.Context(), application.GetAllProductsQuery{})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	responses := make([]ProductResponse, 0, len(dtos))
	for _, dto := range dtos {
		responses = append(responses, productResponseFrom(dto))
	}
	ctx.JSON(http.StatusOK, responses)
}
