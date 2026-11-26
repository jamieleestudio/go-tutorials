// Package impl OrderService 的实现（对应 Java 的 OrderServiceImpl）。
//
// 编排用例、管理事务边界。依赖：
//   - 本上下文的领域仓储端口；
//   - product / payment 的跨服务契约（provider-defined api 接口）——
//     Java 版经 OpenFeign 远程调用，Go 版由 infrastructure/grpc 适配为 gRPC；
//   - 事件发布端口。
//     绝不直接依赖基础设施实现。
package impl

import (
	"context"

	"github.com/jamieleestudio/go-tutorials/architecture/microservices/order/application"
	"github.com/jamieleestudio/go-tutorials/architecture/microservices/order/domain"
	paymentapi "github.com/jamieleestudio/go-tutorials/architecture/microservices/payment/api"
	productapi "github.com/jamieleestudio/go-tutorials/architecture/microservices/product/api"
	"github.com/jamieleestudio/go-tutorials/architecture/microservices/shared"
)

var _ application.OrderService = (*OrderServiceImpl)(nil)

// OrderServiceImpl 应用服务实现。
type OrderServiceImpl struct {
	orderRepository       domain.OrderRepository
	txRunner              shared.TxRunner
	productService        productapi.ProductService
	paymentService        paymentapi.PaymentService
	eventPublisherService application.EventPublisherService
}

func NewOrderServiceImpl(
	orderRepository domain.OrderRepository,
	txRunner shared.TxRunner,
	productService productapi.ProductService,
	paymentService paymentapi.PaymentService,
	eventPublisherService application.EventPublisherService,
) *OrderServiceImpl {
	return &OrderServiceImpl{
		orderRepository:       orderRepository,
		txRunner:              txRunner,
		productService:        productService,
		paymentService:        paymentService,
		eventPublisherService: eventPublisherService,
	}
}

// CreateOrder 创建订单：查商品（远程）→ 组装订单行 → 落库 → 发布事件。
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
		if err := s.publishEvents(order); err != nil {
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

// PayOrder 支付订单：远程 Pay（幂等）→ 订单标记 PAID → 落库 → 发布事件。
func (s *OrderServiceImpl) PayOrder(ctx context.Context, cmd application.PayOrderCommand) error {
	return s.txRunner.WithinTx(ctx, func(ctx context.Context) error {
		order, err := s.findOrderOrThrow(ctx, cmd.OrderID)
		if err != nil {
			return err
		}
		paymentID, err := s.paymentService.Pay(ctx, cmd.OrderID, order.TotalAmount())
		if err != nil {
			return err
		}
		if err := order.MarkAsPaid(paymentID); err != nil {
			return err
		}
		if err := s.orderRepository.Save(ctx, order); err != nil {
			return err
		}
		return s.publishEvents(order)
	})
}

// CancelOrder 取消订单：已支付则远程退款 → 订单标记 CANCELLED → 落库 → 发布事件。
func (s *OrderServiceImpl) CancelOrder(ctx context.Context, cmd application.CancelOrderCommand) error {
	return s.txRunner.WithinTx(ctx, func(ctx context.Context) error {
		order, err := s.findOrderOrThrow(ctx, cmd.OrderID)
		if err != nil {
			return err
		}
		if order.PaymentID() != "" {
			if err := s.paymentService.Refund(ctx, order.PaymentID()); err != nil {
				return err
			}
		}
		if err := order.Cancel(cmd.Reason); err != nil {
			return err
		}
		if err := s.orderRepository.Save(ctx, order); err != nil {
			return err
		}
		return s.publishEvents(order)
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

// publishEvents 发布聚合上的领域事件，发布后清空防止重复发布。
func (s *OrderServiceImpl) publishEvents(order *domain.Order) error {
	for _, e := range order.DomainEvents() {
		s.eventPublisherService.Publish(e)
	}
	order.ClearDomainEvents()
	return nil
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
		OrderID:     order.ID(),
		CustomerID:  order.CustomerID(),
		Status:      order.Status(),
		TotalAmount: order.TotalAmount(),
		Lines:       lines,
	}
}
