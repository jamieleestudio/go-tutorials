// Package impl ProductAppService 的实现（对应 Java 的 ProductAppServiceImpl）。
package impl

import (
	"context"
	"log/slog"

	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/product/api"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/product/application"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/product/domain"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared/event"
)

var _ application.ProductAppService = (*ProductServiceImpl)(nil)

// ProductServiceImpl 应用服务实现。
// 只依赖领域仓储端口与 ports.go 定义的端口 —— 不感知任何框架与持久化细节。
type ProductServiceImpl struct {
	productRepository domain.ProductRepository
	deductedOrders    domain.DeductedOrderRepository
	txRunner          shared.TxRunner
	eventPublisher    application.EventPublisherService
	eventDedup        application.EventDeduplicationService
}

func NewProductServiceImpl(
	productRepository domain.ProductRepository,
	deductedOrders domain.DeductedOrderRepository,
	txRunner shared.TxRunner,
	eventPublisher application.EventPublisherService,
	eventDedup application.EventDeduplicationService,
) *ProductServiceImpl {
	return &ProductServiceImpl{
		productRepository: productRepository,
		deductedOrders:    deductedOrders,
		txRunner:          txRunner,
		eventPublisher:    eventPublisher,
		eventDedup:        eventDedup,
	}
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

// DeductForOrder Saga 反应：OrderCreatedEvent → 扣库存 → 记账 → InventoryDeducted。
//
// 与 Java 完全对应：
//   - @Transactional → WithinTx：扣减、记账、InventoryDeducted 落 outbox、markProcessed
//     全在一个本地事务，要么全提交要么全回滚；
//   - 预校验【全部】行后再统一扣减 —— 失败时零部分扣减（no partial deduction）；
//   - 业务失败（缺品/缺货）被 catch：发布 InventoryDeductFailed 而非向上抛错 ——
//     事务提交，失败事件落 outbox 触发 Saga 补偿；只有发布/标记本身的故障才回滚重试。
func (s *ProductServiceImpl) DeductForOrder(ctx context.Context, eventID string, evt *event.OrderCreatedEvent) error {
	return s.txRunner.WithinTx(ctx, func(ctx context.Context) error {
		duplicate, err := s.eventDedup.IsDuplicate(ctx, eventID)
		if err != nil {
			return err
		}
		if duplicate {
			slog.Info("duplicate OrderCreatedEvent, skipping", "eventId", eventID, "orderId", evt.OrderID)
			return nil
		}

		if err := s.deductForOrder(ctx, evt); err != nil {
			// 对应 Java 的 catch 块：校验/扣减失败 → 发布失败事件（同事务提交）
			slog.Warn("inventory deduction failed, publishing failure event",
				"orderId", evt.OrderID, "error", err)
			if pubErr := s.eventPublisher.Publish(ctx, &event.InventoryDeductFailedEvent{
				OrderID: evt.OrderID,
				Reason:  err.Error(),
			}); pubErr != nil {
				return pubErr // 发布故障：回滚整个事务，等事件重投后重新处理
			}
			return s.eventDedup.MarkProcessed(ctx, eventID)
		}

		slog.Info("inventory deducted for order", "orderId", evt.OrderID)
		return s.eventDedup.MarkProcessed(ctx, eventID)
	})
}

func (s *ProductServiceImpl) deductForOrder(ctx context.Context, evt *event.OrderCreatedEvent) error {
	// 按 ID 缓存聚合实例：同一产品出现在多行时累积扣减
	// （对应 JPA 一级缓存 —— 同一事务内 findById 同一 ID 返回同一实例）。
	products := map[string]*domain.Product{}
	getProduct := func(productID string) (*domain.Product, error) {
		if p, ok := products[productID]; ok {
			return p, nil
		}
		p, err := s.findProductOrThrow(ctx, productID)
		if err != nil {
			return nil, err
		}
		products[productID] = p
		return p, nil
	}

	// 第一遍：预校验全部行（PRODUCT_NOT_FOUND / OUT_OF_STOCK 在这里失败）
	for _, line := range evt.Lines {
		product, err := getProduct(line.ProductID)
		if err != nil {
			return err
		}
		if err := product.DeductStock(line.Quantity); err != nil {
			return err
		}
	}
	// 第二遍：全部校验通过 → 统一落库
	for _, line := range evt.Lines {
		if err := s.productRepository.Save(ctx, products[line.ProductID]); err != nil {
			return err
		}
	}

	// 补偿记账：记住本订单扣了什么 —— 取消时凭它精确回补
	lines := make([]domain.DeductedLine, 0, len(evt.Lines))
	for _, line := range evt.Lines {
		lines = append(lines, domain.DeductedLine{ProductID: line.ProductID, Quantity: line.Quantity})
	}
	if err := s.deductedOrders.Save(ctx, &domain.DeductedOrderRecord{OrderID: evt.OrderID, Lines: lines}); err != nil {
		return err
	}
	return s.eventPublisher.Publish(ctx, &event.InventoryDeductedEvent{OrderID: evt.OrderID})
}

// RestockForOrder Saga 补偿反应：OrderCancelledEvent → 凭记账回补库存。
// 没有记账记录则跳过（幂等：可能已回补过 —— at-least-once 的重复到达）。
func (s *ProductServiceImpl) RestockForOrder(ctx context.Context, eventID string, orderID string) error {
	return s.txRunner.WithinTx(ctx, func(ctx context.Context) error {
		duplicate, err := s.eventDedup.IsDuplicate(ctx, eventID)
		if err != nil {
			return err
		}
		if duplicate {
			slog.Info("duplicate OrderCancelledEvent, skipping", "eventId", eventID, "orderId", orderID)
			return nil
		}

		record, err := s.deductedOrders.FindByID(ctx, orderID)
		if err != nil {
			return err
		}
		if record != nil {
			for _, line := range record.Lines {
				product, err := s.findProductOrThrow(ctx, line.ProductID)
				if err != nil {
					return err
				}
				if err := product.Restock(line.Quantity); err != nil {
					return err
				}
				if err := s.productRepository.Save(ctx, product); err != nil {
					return err
				}
			}
			if err := s.deductedOrders.Delete(ctx, orderID); err != nil {
				return err
			}
			slog.Info("restocked for cancelled order", "orderId", orderID, "lines", len(record.Lines))
		}
		return s.eventDedup.MarkProcessed(ctx, eventID)
	})
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
