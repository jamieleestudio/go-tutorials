// Package impl OrderService 的实现（对应 Java 的 OrderServiceImpl）——
// Saga 的发起方与汇合点。
//
// createOrder：一个本地事务 = 订单落库 + OrderCreatedEvent 落 outbox
// （本地消息表模式），立即返回 —— 支付与扣库存异步进行（最终一致性），
// Saga 经 4 个 on* 事件反应完成。
//
// 依赖：
//   - 本上下文的领域仓储端口；
//   - product 的跨服务契约（同步目录【读】—— 务实的混合模式）；
//     Java 版经 OpenFeign，Go 版由 infrastructure/grpc 适配为 gRPC；
//   - ports.go 的发布 / 去重端口。绝不直接依赖基础设施实现。
package impl

import (
	"context"

	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/order/application"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/order/domain"
	productapi "github.com/jamieleestudio/go-tutorials/architecture/event-driven/product/api"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared/event"
)

var _ application.OrderService = (*OrderServiceImpl)(nil)

// OrderServiceImpl Saga 发起方实现。
type OrderServiceImpl struct {
	orderRepository       domain.OrderRepository
	productService        productapi.ProductService
	txRunner              shared.TxRunner
	eventPublisherService application.EventPublisherService
	eventDeduplication    application.EventDeduplicationService
}

func NewOrderServiceImpl(
	orderRepository domain.OrderRepository,
	productService productapi.ProductService,
	txRunner shared.TxRunner,
	eventPublisherService application.EventPublisherService,
	eventDeduplication application.EventDeduplicationService,
) *OrderServiceImpl {
	return &OrderServiceImpl{
		orderRepository:       orderRepository,
		productService:        productService,
		txRunner:              txRunner,
		eventPublisherService: eventPublisherService,
		eventDeduplication:    eventDeduplication,
	}
}

// CreateOrder 创建订单：查商品（同步 gRPC 读）→ 组装订单行 → 落库 → Outbox。
// 一个本地事务内完成订单 + OrderCreatedEvent 的原子落库 —— 不会丢单。
// 返回即代表 Saga 已启动；CONFIRMED 由后续事件反应达成。
func (s *OrderServiceImpl) CreateOrder(ctx context.Context, cmd application.CreateOrderCommand) (*application.OrderDto, error) {
	var dto *application.OrderDto
	err := s.txRunner.WithinTx(ctx, func(ctx context.Context) error {
		productIDs := make([]string, 0, len(cmd.Lines))
		for _, line := range cmd.Lines {
			productIDs = append(productIDs, line.ProductID)
		}

		products, err := s.productService.GetProducts(ctx, productIDs)
		if err != nil {
			return err
		}

		items := make([]*domain.OrderItem, 0, len(cmd.Lines))
		for _, line := range cmd.Lines {
			product := findProduct(products, line.ProductID)
			if product == nil {
				// 与 Java 一致：批量查询缺失的行在这里升级为 PRODUCT_NOT_FOUND
				return shared.NewEntityNotFound("PRODUCT_NOT_FOUND", "Product not found: "+line.ProductID)
			}
			item, err := domain.NewOrderItem(product.ProductID, product.Name, line.Quantity, product.Price)
			if err != nil {
				return err
			}
			items = append(items, item)
		}

		order, err := domain.Create(cmd.CustomerID, items)
		if err != nil {
			return err
		}
		if err := s.orderRepository.Save(ctx, order); err != nil {
			return err
		}

		// 同一本地事务：订单 + outbox 消息（原子 → 不丢单）
		lines := make([]event.OrderLine, 0, len(order.Items()))
		for _, item := range order.Items() {
			lines = append(lines, event.OrderLine{
				ProductID:   item.ProductID(),
				ProductName: item.ProductName(),
				Quantity:    item.Quantity(),
				UnitPrice:   item.UnitPrice().StringFixed(2),
			})
		}
		if err := s.eventPublisherService.Publish(ctx, &event.OrderCreatedEvent{
			OrderID:     order.ID(),
			CustomerID:  order.CustomerID(),
			Lines:       lines,
			TotalAmount: order.TotalAmount().StringFixed(2),
		}); err != nil {
			return err
		}

		dto = toDto(order)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return dto, nil
}

// CancelOrder 同步取消（REST）：CREATED 才可取消 → 发布 OrderCancelledEvent
// → payment 退款、product 凭记账回补（补偿由事件协同完成）。
func (s *OrderServiceImpl) CancelOrder(ctx context.Context, cmd application.CancelOrderCommand) error {
	return s.txRunner.WithinTx(ctx, func(ctx context.Context) error {
		order, err := s.findOrderOrThrow(ctx, cmd.OrderID)
		if err != nil {
			return err
		}
		if err := order.Cancel(cmd.Reason); err != nil {
			return err
		}
		if err := s.orderRepository.Save(ctx, order); err != nil {
			return err
		}
		return s.eventPublisherService.Publish(ctx, &event.OrderCancelledEvent{
			OrderID: cmd.OrderID,
			Reason:  cmd.Reason,
		})
	})
}

// OnPaymentSucceeded payment-events 流：记录进度 → 双到齐则 CONFIRMED。
func (s *OrderServiceImpl) OnPaymentSucceeded(ctx context.Context, eventID string, orderID string, paymentID string) error {
	return s.txRunner.WithinTx(ctx, func(ctx context.Context) error {
		duplicate, err := s.eventDeduplication.IsDuplicate(ctx, eventID)
		if err != nil {
			return err
		}
		if duplicate {
			return nil
		}
		order, err := s.findOrderOrThrow(ctx, orderID)
		if err != nil {
			return err
		}
		if err := order.MarkPaymentSucceeded(paymentID); err != nil {
			return err
		}
		if err := s.orderRepository.Save(ctx, order); err != nil {
			return err
		}
		return s.eventDeduplication.MarkProcessed(ctx, eventID)
	})
}

// OnPaymentFailed payment-events 流：仅 CREATED → 取消（已 CONFIRMED/CANCELLED 跳过）。
func (s *OrderServiceImpl) OnPaymentFailed(ctx context.Context, eventID string, orderID string, reason string) error {
	return s.txRunner.WithinTx(ctx, func(ctx context.Context) error {
		duplicate, err := s.eventDeduplication.IsDuplicate(ctx, eventID)
		if err != nil {
			return err
		}
		if duplicate {
			return nil
		}
		order, err := s.findOrderOrThrow(ctx, orderID)
		if err != nil {
			return err
		}
		if order.Status() == domain.StatusCreated {
			if err := order.Cancel(reason); err != nil {
				return err
			}
			if err := s.orderRepository.Save(ctx, order); err != nil {
				return err
			}
			// 补偿发布：product 凭记账回补库存
			if err := s.eventPublisherService.Publish(ctx, &event.OrderCancelledEvent{
				OrderID: orderID,
				Reason:  reason,
			}); err != nil {
				return err
			}
		}
		return s.eventDeduplication.MarkProcessed(ctx, eventID)
	})
}

// OnInventoryDeducted product-events 流：记录进度 → 双到齐则 CONFIRMED。
func (s *OrderServiceImpl) OnInventoryDeducted(ctx context.Context, eventID string, orderID string) error {
	return s.txRunner.WithinTx(ctx, func(ctx context.Context) error {
		duplicate, err := s.eventDeduplication.IsDuplicate(ctx, eventID)
		if err != nil {
			return err
		}
		if duplicate {
			return nil
		}
		order, err := s.findOrderOrThrow(ctx, orderID)
		if err != nil {
			return err
		}
		if err := order.MarkInventoryDeducted(); err != nil {
			return err
		}
		if err := s.orderRepository.Save(ctx, order); err != nil {
			return err
		}
		return s.eventDeduplication.MarkProcessed(ctx, eventID)
	})
}

// OnInventoryDeductFailed product-events 流：仅 CREATED → 取消（缺货补偿）。
func (s *OrderServiceImpl) OnInventoryDeductFailed(ctx context.Context, eventID string, orderID string, reason string) error {
	return s.txRunner.WithinTx(ctx, func(ctx context.Context) error {
		duplicate, err := s.eventDeduplication.IsDuplicate(ctx, eventID)
		if err != nil {
			return err
		}
		if duplicate {
			return nil
		}
		order, err := s.findOrderOrThrow(ctx, orderID)
		if err != nil {
			return err
		}
		if order.Status() == domain.StatusCreated {
			if err := order.Cancel(reason); err != nil {
				return err
			}
			if err := s.orderRepository.Save(ctx, order); err != nil {
				return err
			}
			if err := s.eventPublisherService.Publish(ctx, &event.OrderCancelledEvent{
				OrderID: orderID,
				Reason:  reason,
			}); err != nil {
				return err
			}
		}
		return s.eventDeduplication.MarkProcessed(ctx, eventID)
	})
}

func (s *OrderServiceImpl) GetOrderByID(ctx context.Context, query application.GetOrderByIDQuery) (*application.OrderDto, error) {
	order, err := s.findOrderOrThrow(ctx, query.OrderID)
	if err != nil {
		return nil, err
	}
	return toDto(order), nil
}

func (s *OrderServiceImpl) GetOrdersByCustomer(ctx context.Context, query application.GetOrdersByCustomerQuery) ([]*application.OrderDto, error) {
	orders, err := s.orderRepository.FindByCustomerID(ctx, query.CustomerID)
	if err != nil {
		return nil, err
	}
	dtos := make([]*application.OrderDto, 0, len(orders))
	for _, order := range orders {
		dtos = append(dtos, toDto(order))
	}
	return dtos, nil
}

func (s *OrderServiceImpl) findOrderOrThrow(ctx context.Context, orderID string) (*domain.Order, error) {
	order, err := s.orderRepository.FindByID(ctx, orderID)
	if err != nil {
		if err == domain.ErrOrderNotFound {
			return nil, shared.NewEntityNotFound("ORDER_NOT_FOUND", "Order not found: "+orderID)
		}
		return nil, err
	}
	return order, nil
}

func findProduct(products []*productapi.ProductDto, productID string) *productapi.ProductDto {
	for _, p := range products {
		if p.ProductID == productID {
			return p
		}
	}
	return nil
}

func toDto(order *domain.Order) *application.OrderDto {
	lines := make([]application.OrderLineDto, 0, len(order.Items()))
	for _, item := range order.Items() {
		lines = append(lines, application.OrderLineDto{
			ProductID:   item.ProductID(),
			ProductName: item.ProductName(),
			Quantity:    item.Quantity(),
			UnitPrice:   item.UnitPrice(),
		})
	}
	return &application.OrderDto{
		OrderID:           order.ID(),
		CustomerID:        order.CustomerID(),
		Status:            order.Status(),
		TotalAmount:       order.TotalAmount(),
		PaymentSucceeded:  order.PaymentSucceeded(),
		InventoryDeducted: order.InventoryDeducted(),
		Lines:             lines,
	}
}
