// Package impl 应用服务实现（对应 Java 的 application.impl 包）。
//
// 编排用例：调用领域对象 + 跨上下文服务接口，管理事务边界，发布领域事件。
// 只依赖：本上下文的 domain、其他上下文的 application（接口层）、shared —— 不碰基础设施。
package impl

import (
	"context"
	"errors"

	orderapp "github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/order/application"
	orderdomain "github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/order/domain"
	paymentapp "github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/payment/application"
	productapp "github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/product/application"
	"github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/shared"
)

// 编译期断言：确保实现没有偏离接口签名。
var _ orderapp.OrderService = (*OrderServiceImpl)(nil)

// OrderServiceImpl 对应 Java 的 OrderServiceImpl。
type OrderServiceImpl struct {
	orderRepo      orderdomain.OrderRepository
	productService productapp.ProductService // 跨上下文：只依赖对方应用层接口
	paymentService paymentapp.PaymentService
	eventPublisher orderapp.EventPublisher
	txRunner       shared.TxRunner // 对应 @Transactional
}

func NewOrderService(
	orderRepo orderdomain.OrderRepository,
	productService productapp.ProductService,
	paymentService paymentapp.PaymentService,
	eventPublisher orderapp.EventPublisher,
	txRunner shared.TxRunner,
) *OrderServiceImpl {
	return &OrderServiceImpl{
		orderRepo:      orderRepo,
		productService: productService,
		paymentService: paymentService,
		eventPublisher: eventPublisher,
		txRunner:       txRunner,
	}
}

// CreateOrder 创建订单：查产品 → 构建订单行 → 创建聚合 → 落库 → 发事件。
func (s *OrderServiceImpl) CreateOrder(ctx context.Context, cmd orderapp.CreateOrderCommand) (*orderapp.OrderDto, error) {
	var dto *orderapp.OrderDto

	// Java 版的 @Transactional → 显式包事务；事务句柄随 ctx 传给下游仓储。
	err := s.txRunner.WithinTx(ctx, func(ctx context.Context) error {
		productIDs := make([]string, 0, len(cmd.Lines))
		for _, line := range cmd.Lines {
			productIDs = append(productIDs, line.ProductID)
		}
		products, err := s.productService.GetProducts(ctx, productIDs)
		if err != nil {
			return err
		}
		byID := make(map[string]productapp.ProductDto, len(products))
		for _, p := range products {
			byID[p.ProductID] = p
		}

		items := make([]orderdomain.OrderItem, 0, len(cmd.Lines))
		for _, line := range cmd.Lines {
			p, ok := byID[line.ProductID]
			if !ok {
				return shared.NewEntityNotFound("PRODUCT_NOT_FOUND", "Product not found: "+line.ProductID)
			}
			item, err := orderdomain.NewOrderItem(p.ProductID, p.Name, line.Quantity, p.Price)
			if err != nil {
				return err
			}
			items = append(items, item)
		}

		order, err := orderdomain.Create(cmd.CustomerID, items)
		if err != nil {
			return err
		}
		if err := s.orderRepo.Save(ctx, order); err != nil {
			return err
		}
		if err := s.publishEvents(ctx, order); err != nil {
			return err
		}
		dto = toOrderDto(order)
		return nil
	})
	return dto, err
}

// PayOrder 支付订单：调 Payment 上下文支付 → 订单标记已支付 → 落库 → 发事件。
func (s *OrderServiceImpl) PayOrder(ctx context.Context, cmd orderapp.PayOrderCommand) error {
	return s.txRunner.WithinTx(ctx, func(ctx context.Context) error {
		order, err := s.findOrder(ctx, cmd.OrderID)
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
		if err := s.orderRepo.Save(ctx, order); err != nil {
			return err
		}
		return s.publishEvents(ctx, order)
	})
}

// CancelOrder 取消订单：已支付则先退款 → 取消聚合 → 落库 → 发事件。
func (s *OrderServiceImpl) CancelOrder(ctx context.Context, cmd orderapp.CancelOrderCommand) error {
	return s.txRunner.WithinTx(ctx, func(ctx context.Context) error {
		order, err := s.findOrder(ctx, cmd.OrderID)
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
		if err := s.orderRepo.Save(ctx, order); err != nil {
			return err
		}
		return s.publishEvents(ctx, order)
	})
}

func (s *OrderServiceImpl) GetOrderByID(ctx context.Context, query orderapp.GetOrderByIDQuery) (*orderapp.OrderDto, error) {
	order, err := s.findOrder(ctx, query.OrderID)
	if err != nil {
		return nil, err
	}
	return toOrderDto(order), nil
}

func (s *OrderServiceImpl) GetOrdersByCustomer(ctx context.Context, query orderapp.GetOrdersByCustomerQuery) ([]orderapp.OrderDto, error) {
	orders, err := s.orderRepo.FindByCustomerID(ctx, query.CustomerID)
	if err != nil {
		return nil, err
	}
	dtos := make([]orderapp.OrderDto, 0, len(orders))
	for _, o := range orders {
		dtos = append(dtos, *toOrderDto(o))
	}
	return dtos, nil
}

// findOrder 对应 Java 的 findOrderOrThrow：
// 仓储返回哨兵错误 ErrOrderNotFound → 转换为 EntityNotFound。
func (s *OrderServiceImpl) findOrder(ctx context.Context, orderID string) (*orderdomain.Order, error) {
	order, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, orderdomain.ErrOrderNotFound) {
			return nil, shared.NewEntityNotFound("ORDER_NOT_FOUND", "Order not found: "+orderID)
		}
		return nil, err
	}
	return order, nil
}

func (s *OrderServiceImpl) publishEvents(ctx context.Context, order *orderdomain.Order) error {
	for _, e := range order.DomainEvents() {
		if err := s.eventPublisher.Publish(ctx, e); err != nil {
			return err
		}
	}
	order.ClearDomainEvents()
	return nil
}

func toOrderDto(o *orderdomain.Order) *orderapp.OrderDto {
	items := o.Items()
	lines := make([]orderapp.OrderLineDto, 0, len(items))
	for _, it := range items {
		lines = append(lines, orderapp.OrderLineDto{
			ProductID:   it.ProductID(),
			ProductName: it.ProductName(),
			Quantity:    it.Quantity(),
			UnitPrice:   it.UnitPrice(),
		})
	}
	return &orderapp.OrderDto{
		OrderID:     o.ID(),
		CustomerID:  o.CustomerID(),
		Status:      o.Status(),
		TotalAmount: o.TotalAmount(),
		Lines:       lines,
	}
}
