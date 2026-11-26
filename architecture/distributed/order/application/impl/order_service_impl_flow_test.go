// 应用层编排的流程测试（对应 Java 的 OrderServiceFlowTest）。
//
// Java 用 @MockBean mock 掉远程的 PaymentService/ProductService ——
// provider-defined 接口让跨服务测试完全不需要真实进程。
// Go 版同样只注入 fake：仓储用真 SQLite 内存库，编排逻辑
// （远程契约调用 + 状态推进 + 事件发布）全走真实代码。
package impl_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/jamieleestudio/go-tutorials/architecture/distributed/order/application"
	"github.com/jamieleestudio/go-tutorials/architecture/distributed/order/application/impl"
	orderinf "github.com/jamieleestudio/go-tutorials/architecture/distributed/order/infrastructure"
	paymentapi "github.com/jamieleestudio/go-tutorials/architecture/distributed/payment/api"
	productapi "github.com/jamieleestudio/go-tutorials/architecture/distributed/product/api"
	"github.com/jamieleestudio/go-tutorials/architecture/distributed/shared"
	"github.com/jamieleestudio/go-tutorials/architecture/distributed/shared/persistence"
)

// ---- fakes（对应 Java 的 @MockBean）----

// fakeProductService product 契约的 fake：固定商品目录。
type fakeProductService struct {
	products []*productapi.ProductDto
}

var (
	_ productapi.ProductService         = (*fakeProductService)(nil)
	_ paymentapi.PaymentService         = (*fakePaymentService)(nil)
	_ application.EventPublisherService = (*fakePublisher)(nil)
)

func (f *fakeProductService) GetProduct(_ context.Context, productID string) (*productapi.ProductDto, error) {
	for _, p := range f.products {
		if p.ProductID == productID {
			return p, nil
		}
	}
	return nil, shared.NewEntityNotFound("PRODUCT_NOT_FOUND", "Product not found: "+productID)
}

func (f *fakeProductService) GetProducts(_ context.Context, productIDs []string) ([]*productapi.ProductDto, error) {
	found := make([]*productapi.ProductDto, 0)
	for _, p := range f.products {
		for _, id := range productIDs {
			if p.ProductID == id {
				found = append(found, p)
			}
		}
	}
	return found, nil
}

// fakePaymentService payment 契约的 fake：像真服务一样按 orderId 幂等。
type fakePaymentService struct {
	payments map[string]string // orderID → paymentID
	refunded []string
}

func (f *fakePaymentService) Pay(_ context.Context, orderID string, _ decimal.Decimal) (string, error) {
	if id, ok := f.payments[orderID]; ok {
		return id, nil // 幂等：同一订单重复支付返回同一支付单 ID
	}
	id := "payment-" + strconv.Itoa(len(f.payments)+1)
	f.payments[orderID] = id
	return id, nil
}

func (f *fakePaymentService) Refund(_ context.Context, paymentID string) error {
	f.refunded = append(f.refunded, paymentID)
	return nil
}

// fakePublisher 事件端口 fake：记录发布的事件。
type fakePublisher struct {
	events []shared.DomainEvent
}

func (f *fakePublisher) Publish(event shared.DomainEvent) {
	f.events = append(f.events, event)
}

func (f *fakePublisher) eventNames() []string {
	names := make([]string, 0, len(f.events))
	for _, e := range f.events {
		names = append(names, e.EventName())
	}
	return names
}

// ---- 装配：真 SQLite 内存库 + 真编排逻辑 + fake 远程契约 ----

func newFlowService(t *testing.T) (application.OrderService, *fakePaymentService, *fakePublisher) {
	t.Helper()
	db, err := persistence.Open("sqlite://file:flow-order?mode=memory&cache=shared", "silent")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := persistence.Migrate(db, &orderinf.OrderEntity{}, &orderinf.OrderItemEntity{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	products := &fakeProductService{products: []*productapi.ProductDto{{
		ProductID: "p-1", Name: "Laptop", Price: decimal.RequireFromString("999.99"), Stock: 10,
	}}}
	payments := &fakePaymentService{payments: make(map[string]string)}
	publisher := &fakePublisher{}
	svc := impl.NewOrderServiceImpl(
		orderinf.NewOrderRepository(db),
		persistence.NewGormTxRunner(db),
		products,
		payments,
		publisher,
	)
	return svc, payments, publisher
}

func createLaptopOrder(t *testing.T, svc application.OrderService) *application.OrderDto {
	t.Helper()
	dto, err := svc.CreateOrder(context.Background(), application.CreateOrderCommand{
		CustomerID: "customer-1",
		Lines:      []application.OrderLine{{ProductID: "p-1", Quantity: 2}},
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	return dto
}

// ---- 测试 ----

// 对应 Java 的 create_and_pay_order_with_remote_services_mocked。
func TestCreateAndPayOrderWithRemoteServicesMocked(t *testing.T) {
	svc, payments, publisher := newFlowService(t)

	created := createLaptopOrder(t, svc)
	if string(created.Status) != "CREATED" {
		t.Fatalf("status = %s, want CREATED", created.Status)
	}
	if created.TotalAmount.StringFixed(2) != "1999.98" {
		t.Fatalf("totalAmount = %s, want 1999.98", created.TotalAmount.StringFixed(2))
	}
	if names := publisher.eventNames(); len(names) != 1 || names[0] != "OrderCreatedEvent" {
		t.Fatalf("events after create = %v, want [OrderCreatedEvent]", names)
	}

	if err := svc.PayOrder(context.Background(), application.PayOrderCommand{OrderID: created.OrderID}); err != nil {
		t.Fatalf("pay order: %v", err)
	}

	paid, err := svc.GetOrderByID(context.Background(), application.GetOrderByIDQuery{OrderID: created.OrderID})
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if string(paid.Status) != "PAID" {
		t.Fatalf("status = %s, want PAID", paid.Status)
	}
	if got := payments.payments[created.OrderID]; got != "payment-1" {
		t.Fatalf("remote pay got orderId=%s → payment %q, want payment-1", created.OrderID, got)
	}
	if names := publisher.eventNames(); len(names) != 2 || names[1] != "OrderPaidEvent" {
		t.Fatalf("events after pay = %v, want [... OrderPaidEvent]", names)
	}
}

// 取消已支付订单 → 编排层先远程退款，再推进状态（对应 Java 的 cancel 流程）。
func TestCancelPaidOrderRefundsViaPaymentService(t *testing.T) {
	svc, payments, publisher := newFlowService(t)

	created := createLaptopOrder(t, svc)
	if err := svc.PayOrder(context.Background(), application.PayOrderCommand{OrderID: created.OrderID}); err != nil {
		t.Fatalf("pay order: %v", err)
	}
	paymentID := payments.payments[created.OrderID]

	if err := svc.CancelOrder(context.Background(), application.CancelOrderCommand{
		OrderID: created.OrderID, Reason: "changed my mind",
	}); err != nil {
		t.Fatalf("cancel order: %v", err)
	}

	cancelled, err := svc.GetOrderByID(context.Background(), application.GetOrderByIDQuery{OrderID: created.OrderID})
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if string(cancelled.Status) != "CANCELLED" {
		t.Fatalf("status = %s, want CANCELLED", cancelled.Status)
	}
	if len(payments.refunded) != 1 || payments.refunded[0] != paymentID {
		t.Fatalf("refunded = %v, want [%s]", payments.refunded, paymentID)
	}
	names := publisher.eventNames()
	if len(names) != 3 || names[2] != "OrderCancelledEvent" {
		t.Fatalf("events after cancel = %v, want [... OrderCancelledEvent]", names)
	}
}
