// Package impl ProductAppService 的实现（对应 Java 的 ProductServiceImpl）。
package impl

import (
	"context"

	"github.com/jamieleestudio/go-tutorials/architecture/cloud-native/product/api"
	"github.com/jamieleestudio/go-tutorials/architecture/cloud-native/product/application"
	"github.com/jamieleestudio/go-tutorials/architecture/cloud-native/product/domain"
	"github.com/jamieleestudio/go-tutorials/architecture/cloud-native/shared"
)

var _ application.ProductAppService = (*ProductServiceImpl)(nil)

// ProductServiceImpl 应用服务实现。
// 只依赖领域仓储端口 —— 不感知任何框架与持久化细节。
type ProductServiceImpl struct {
	productRepository domain.ProductRepository
}

func NewProductServiceImpl(productRepository domain.ProductRepository) *ProductServiceImpl {
	return &ProductServiceImpl{productRepository: productRepository}
}

func (s *ProductServiceImpl) CreateProduct(ctx context.Context, cmd application.CreateProductCommand) (*api.ProductDto, error) {
	product, err := domain.Create(cmd.Name, cmd.Price, cmd.Stock)
	if err != nil {
		return nil, err
	}
	if err := s.productRepository.Save(ctx, product); err != nil {
		return nil, err
	}
	return toDto(product), nil
}

func (s *ProductServiceImpl) DeductStock(ctx context.Context, cmd application.DeductStockCommand) error {
	product, err := s.findProductOrThrow(ctx, cmd.ProductID)
	if err != nil {
		return err
	}
	if err := product.DeductStock(cmd.Quantity); err != nil {
		return err
	}
	return s.productRepository.Save(ctx, product)
}

func (s *ProductServiceImpl) GetProductByID(ctx context.Context, query application.GetProductByIDQuery) (*api.ProductDto, error) {
	product, err := s.findProductOrThrow(ctx, query.ProductID)
	if err != nil {
		return nil, err
	}
	return toDto(product), nil
}

func (s *ProductServiceImpl) GetAllProducts(ctx context.Context, _ application.GetAllProductsQuery) ([]*api.ProductDto, error) {
	products, err := s.productRepository.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	return toDtos(products), nil
}

// GetProduct 跨服务契约方法（gRPC 暴露给 order 服务）。
func (s *ProductServiceImpl) GetProduct(ctx context.Context, productID string) (*api.ProductDto, error) {
	product, err := s.findProductOrThrow(ctx, productID)
	if err != nil {
		return nil, err
	}
	return toDto(product), nil
}

// GetProducts 跨服务契约方法。与 Java 一致：缺失的 ID 不报错，只返回找到的。
func (s *ProductServiceImpl) GetProducts(ctx context.Context, productIDs []string) ([]*api.ProductDto, error) {
	products, err := s.productRepository.FindAllByIDs(ctx, productIDs)
	if err != nil {
		return nil, err
	}
	return toDtos(products), nil
}

func (s *ProductServiceImpl) findProductOrThrow(ctx context.Context, productID string) (*domain.Product, error) {
	product, err := s.productRepository.FindByID(ctx, productID)
	if err != nil {
		if err == domain.ErrProductNotFound {
			return nil, shared.NewEntityNotFound("PRODUCT_NOT_FOUND", "Product not found: "+productID)
		}
		return nil, err
	}
	return product, nil
}

func toDto(product *domain.Product) *api.ProductDto {
	return &api.ProductDto{
		ProductID: product.ID(),
		Name:      product.Name(),
		Price:     product.Price(),
		Stock:     product.Stock(),
	}
}

func toDtos(products []*domain.Product) []*api.ProductDto {
	dtos := make([]*api.ProductDto, 0, len(products))
	for _, p := range products {
		dtos = append(dtos, toDto(p))
	}
	return dtos
}
