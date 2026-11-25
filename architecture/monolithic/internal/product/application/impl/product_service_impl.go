// Package impl 产品应用服务实现（对应 Java 的 ProductServiceImpl）。
package impl

import (
	"context"
	"errors"

	productapp "github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/product/application"
	productdomain "github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/product/domain"
	"github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/shared"
)

var _ productapp.ProductService = (*ProductServiceImpl)(nil)

// ProductServiceImpl 产品应用服务实现。
type ProductServiceImpl struct {
	productRepo productdomain.ProductRepository
	txRunner    shared.TxRunner
}

func NewProductService(productRepo productdomain.ProductRepository, txRunner shared.TxRunner) *ProductServiceImpl {
	return &ProductServiceImpl{productRepo: productRepo, txRunner: txRunner}
}

func (s *ProductServiceImpl) CreateProduct(ctx context.Context, cmd productapp.CreateProductCommand) (*productapp.ProductDto, error) {
	var dto *productapp.ProductDto
	err := s.txRunner.WithinTx(ctx, func(ctx context.Context) error {
		product, err := productdomain.Create(cmd.Name, cmd.Price, cmd.Stock)
		if err != nil {
			return err
		}
		if err := s.productRepo.Save(ctx, product); err != nil {
			return err
		}
		dto = toProductDto(product)
		return nil
	})
	return dto, err
}

func (s *ProductServiceImpl) DeductStock(ctx context.Context, cmd productapp.DeductStockCommand) error {
	return s.txRunner.WithinTx(ctx, func(ctx context.Context) error {
		product, err := s.findProduct(ctx, cmd.ProductID)
		if err != nil {
			return err
		}
		if err := product.DeductStock(cmd.Quantity); err != nil {
			return err
		}
		return s.productRepo.Save(ctx, product)
	})
}

func (s *ProductServiceImpl) GetProductByID(ctx context.Context, query productapp.GetProductByIDQuery) (*productapp.ProductDto, error) {
	product, err := s.findProduct(ctx, query.ProductID)
	if err != nil {
		return nil, err
	}
	return toProductDto(product), nil
}

func (s *ProductServiceImpl) GetAllProducts(ctx context.Context, _ productapp.GetAllProductsQuery) ([]productapp.ProductDto, error) {
	products, err := s.productRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	return toProductDtos(products), nil
}

func (s *ProductServiceImpl) GetProduct(ctx context.Context, productID string) (*productapp.ProductDto, error) {
	return s.GetProductByID(ctx, productapp.GetProductByIDQuery{ProductID: productID})
}

func (s *ProductServiceImpl) GetProducts(ctx context.Context, productIDs []string) ([]productapp.ProductDto, error) {
	products, err := s.productRepo.FindAllByIDs(ctx, productIDs)
	if err != nil {
		return nil, err
	}
	return toProductDtos(products), nil
}

func (s *ProductServiceImpl) findProduct(ctx context.Context, productID string) (*productdomain.Product, error) {
	product, err := s.productRepo.FindByID(ctx, productID)
	if err != nil {
		if errors.Is(err, productdomain.ErrProductNotFound) {
			return nil, shared.NewEntityNotFound("PRODUCT_NOT_FOUND", "Product not found: "+productID)
		}
		return nil, err
	}
	return product, nil
}

func toProductDto(p *productdomain.Product) *productapp.ProductDto {
	return &productapp.ProductDto{
		ProductID: p.ID(),
		Name:      p.Name(),
		Price:     p.Price(),
		Stock:     p.Stock(),
	}
}

func toProductDtos(products []*productdomain.Product) []productapp.ProductDto {
	dtos := make([]productapp.ProductDto, 0, len(products))
	for _, p := range products {
		dtos = append(dtos, *toProductDto(p))
	}
	return dtos
}
