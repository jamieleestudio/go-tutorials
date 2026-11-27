// Package web 商品服务的 REST 接口层（对应 Java 的 ProductController）。
package web

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jamieleestudio/go-tutorials/architecture/cloud-native/product/api"
)

// ProductRpcController product 服务暴露给 order 服务的内部 RPC 契约。
//
// 对应 Java 的 ProductRpcController：
//   - GET /rpc/products/{productId} → ProductDto JSON；
//   - GET /rpc/products?ids=a,b     → []ProductDto JSON。
type ProductRpcController struct {
	products api.ProductService
}

func NewProductRpcController(products api.ProductService) *ProductRpcController {
	return &ProductRpcController{products: products}
}

// Register 挂载 RPC 端点。
func (c *ProductRpcController) Register(r gin.IRouter) {
	g := r.Group("/rpc/products")
	g.GET("", c.getProducts)
	g.GET("/:productId", c.getProduct)
}

func (c *ProductRpcController) getProduct(ctx *gin.Context) {
	dto, err := c.products.GetProduct(ctx.Request.Context(), ctx.Param("productId"))
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusOK, dto)
}

// getProducts 批量查询：ids 逗号分隔（对应 Java @RequestParam List<String> 的自动拆分）。
func (c *ProductRpcController) getProducts(ctx *gin.Context) {
	var ids []string
	for _, part := range strings.Split(ctx.Query("ids"), ",") {
		if s := strings.TrimSpace(part); s != "" {
			ids = append(ids, s)
		}
	}
	if len(ids) == 0 {
		ctx.JSON(http.StatusOK, []*api.ProductDto{})
		return
	}
	dtos, err := c.products.GetProducts(ctx.Request.Context(), ids)
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusOK, dtos)
}
