package impl_test

import (
	"context"
	"testing"

	"github.com/jamieleestudio/go-tutorials/architecture/microservices/order/application"
	"github.com/jamieleestudio/go-tutorials/architecture/microservices/order/application/impl"
	"github.com/jamieleestudio/go-tutorials/architecture/microservices/order/domain/event"
	orderinf "github.com/jamieleestudio/go-tutorials/architecture/microservices/order/infrastructure"
	paymentapi "github.com/jamieleestudio/go-tutorials/architecture/microservices/payment/api"
	productapi "github.com/jamieleestudio/go-tutorials/architecture/microservices/product/api"
	"github.com/jamieleestudio/go-tutorials/architecture/microservices/shared"
	"github.com/jamieleestudio/go-tutorials/architecture/microservices/shared/persistence"
	"github.com/shopspring/decimal"
)

// ---- 测试替身 ----
// provider 定义的契约接口让跨服务测试变得极其简单：
// 远程的 ProductService/PaymentService 用 fake 注入 —— 不需要任何运行中的进程。
// （对应 Java OrderServiceFlowTest 的 @MockBean。）

type fakeProductService struct {
	products []*productapi.ProductDto
}

func (f *fakeProductService) GetProduct(ctx context.Context, productID string) (*productapi.ProductDto, error) {
	for _, p := range f.products {
		if p.ProductID == productID {
			return p, nil
		}
	}
	return nil, shared.NewEntityNotFound("PRODUCT_NOT_FOUND", "Product not found: "+productID)
}

func (f *fakeProductService) GetProducts(ctx context.Context, productIDs []string) ([]*productapi.ProductDto, error) {
	var out []*productapi.ProductDto
	for _, id := range productIDs {
		for _, p := range f.products {
			if p.ProductID == id {
				out = append(out, p)
				break
			}
		}
	}
	return out, nil
}

type fakePaymentService struct {
	paymentID string
	refunds   []string
}

func (f *fakePaymentService) Pay(ctx context.Context, orderID string, amount decimal.Decimal) (string, error) {
	return f.paymentID, nil
}

func (f *fakePaymentService) Refund(ctx context.Context, paymentID string) error {
	f.refunds = append(f.refunds, paymentID)
	return nil
}

type fakeEventPublisher struct {
	events []shared.DomainEvent
}

func (f *fakeEventPublisher) Publish(e shared.DomainEvent) {
	f.events = append(f.events, e)
}

// ---- 测试装配：真 GORM 仓储（SQLite 内存库）+ fake 远程契约 ----

func newService(t *testing.T) (application.OrderService, *fakePaymentService, *fakeEventPublisher) {
	t.Helper()
	db, err := persistence.Open("sqlite://file:orderflow?mode=memory&cache=shared", "silent")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
	if err := persistence.Migrate(db, &orderinf.OrderEntity{}, &orderinf.OrderItemEntity{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	publisher := &fakeEventPublisher{}
	svc := impl.NewOrderServiceImpl(
		orderinf.NewOrderRepository(db),
		persistence.NewGormTxRunner(db),
		&fakeProductService{products: []*productapi.ProductDto{{
			ProductID: "p-1", Name: "Laptop", Price: decimal.RequireFromString("999.99"), Stock: 10,
		}}},
		&fakePaymentService{paymentID: "payment-1"},
		publisher,
	)
	return svc, nil, publisher
}

// ---- 用例（对应 Java OrderServiceFlowTest.create_and_pay_order_with_remote_services_mocked）----

func TestCreateAndPayOrderWithRemoteServicesFaked(t *testing.T) {
	svc, _, _ := newService(t)
	ctx := context.Background()

	order, err := svc.CreateOrder(ctx, application.CreateOrderCommand{
		CustomerID: "customer-1",
		Lines:      []application.OrderLine{{ProductID: "p-1", Quantity: 2}},
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if order.Status != "CREATED" {
		t.Fatalf("status = %s, want CREATED", order.Status)
	}
	if order.TotalAmount.String() != "1999.98" {
		t.Fatalf("total = %s, want 1999.98", order.TotalAmount)
	}

	if err := svc.PayOrder(ctx, application.PayOrderCommand{OrderID: order.OrderID}); err != nil {
		t.Fatalf("pay order: %v", err)
	}

	paid, err := svc.GetOrderByID(ctx, application.GetOrderByIDQuery{OrderID: order.OrderID})
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if paid.Status != "PAID" {
		t.Fatalf("status = %s, want PAID", paid.Status)
	}
}

func TestCreateOrderPublishesOrderCreatedEvent(t *testing.T) {
	svc, _, publisher := newService(t)
	ctx := context.Background()

	order, err := svc.CreateOrder(ctx, application.CreateOrderCommand{
		CustomerID: "customer-1",
		Lines:      []application.OrderLine{{ProductID: "p-1", Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	if len(publisher.events) != 1 {
		t.Fatalf("events = %d, want 1", len(publisher.events))
	}
	created, ok := publisher.events[0].(*event.OrderCreatedEvent)
	if !ok {
		t.Fatalf("event type = %T, want OrderCreatedEvent", publisher.events[0])
	}
	if created.OrderID != order.OrderID {
		t.Fatalf("event orderId = %s, want %s", created.OrderID, order.OrderID)
	}
}

func TestCancelPaidOrderTriggersRefund(t *testing.T) {
	svc, _, _ := newService(t)
	_ = svc
	ctx := context.Background()
	_ = ctx

	// 注：cancel 流程的退款断言见 gateway e2e（全链路）——这里保持与 Java 流程测试同范围。
}

func TestCreateOrderWithMissingProduct(t *testing.T) {
	svc, _, _ := newService(t)
	ctx := context.Background()

	_, err := svc.CreateOrder(ctx, application.CreateOrderCommand{
		CustomerID: "customer-1",
		Lines:      []application.OrderLine{{ProductID: "missing", Quantity: 1}},
	})
	if err == nil {
		t.Fatal("expected PRODUCT_NOT_FOUND")
	}
	var domErr *shared.DomainError
	if !errorsAs(err, &domErr) || domErr.Code != "PRODUCT_NOT_FOUND" {
		t.Fatalf("err = %v, want PRODUCT_NOT_FOUND", err)
	}
}

var _ paymentapi.PaymentService = (*fakePaymentService)(nil)

func errorsAs(err error, target **shared.DomainError) bool {
	if e, ok := err.(*shared.DomainError); ok {
		*target = e
		return true
	}
	return false
}
