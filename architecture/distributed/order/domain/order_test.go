package domain

import (
	"testing"

	"github.com/jamieleestudio/go-tutorials/architecture/distributed/order/domain/event"
	"github.com/jamieleestudio/go-tutorials/architecture/distributed/shared"
	"github.com/shopspring/decimal"
)

func item(t *testing.T, productID string, qty int, price string) *OrderItem {
	t.Helper()
	it, err := NewOrderItem(productID, "Product "+productID, qty, mustDecimal(t, price))
	if err != nil {
		t.Fatalf("new order item: %v", err)
	}
	return it
}

func mustDecimal(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	if err != nil {
		t.Fatalf("bad decimal %q: %v", s, err)
	}
	return d
}

func TestCreateOrder(t *testing.T) {
	order, err := Create("customer-1", []*OrderItem{
		item(t, "p-1", 2, "999.99"),
		item(t, "p-2", 1, "0.02"),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if order.Status() != StatusCreated {
		t.Fatalf("status = %s, want CREATED", order.Status())
	}
	if order.TotalAmount().StringFixed(2) != "2000.00" {
		t.Fatalf("total = %s, want 2000.00", order.TotalAmount())
	}

	events := order.DomainEvents()
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	created, ok := events[0].(*event.OrderCreatedEvent)
	if !ok {
		t.Fatalf("event type = %T, want OrderCreatedEvent", events[0])
	}
	if created.AggregateID() != order.ID() {
		t.Fatalf("event aggregateId = %s, want %s", created.AggregateID(), order.ID())
	}
}

func TestCreateOrderValidatesFields(t *testing.T) {
	if _, err := Create("", []*OrderItem{item(t, "p-1", 1, "1")}); err == nil {
		t.Fatal("expected error for empty customerId")
	}
	if _, err := Create("c-1", nil); err == nil {
		t.Fatal("expected error for empty items")
	}
}

func TestOrderItemValidation(t *testing.T) {
	if _, err := NewOrderItem("", "name", 1, mustDecimal(t, "1")); err == nil {
		t.Fatal("expected error for empty productId")
	}
	if _, err := NewOrderItem("p-1", "name", 0, mustDecimal(t, "1")); err == nil {
		t.Fatal("expected error for zero quantity")
	}
	if _, err := NewOrderItem("p-1", "name", 1, mustDecimal(t, "-1")); err == nil {
		t.Fatal("expected error for negative price")
	}
}

func TestMarkAsPaid(t *testing.T) {
	order, _ := Create("c-1", []*OrderItem{item(t, "p-1", 1, "10")})
	order.ClearDomainEvents()

	if err := order.MarkAsPaid("payment-1"); err != nil {
		t.Fatalf("mark as paid: %v", err)
	}
	if order.Status() != StatusPaid || order.PaymentID() != "payment-1" {
		t.Fatalf("status=%s paymentId=%s", order.Status(), order.PaymentID())
	}
	events := order.DomainEvents()
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1 (OrderPaidEvent)", len(events))
	}
	if _, ok := events[0].(*event.OrderPaidEvent); !ok {
		t.Fatalf("event type = %T", events[0])
	}

	// PAID 不能再支付
	err := order.MarkAsPaid("payment-2")
	if err == nil {
		t.Fatal("expected INVALID_TRANSITION for double pay")
	}
	if e, ok := err.(*shared.DomainError); !ok || e.Code != "INVALID_TRANSITION" {
		t.Fatalf("err = %v, want INVALID_TRANSITION", err)
	}
}

func TestCancelOrder(t *testing.T) {
	order, _ := Create("c-1", []*OrderItem{item(t, "p-1", 1, "10")})
	if err := order.Cancel("changed mind"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if order.Status() != StatusCancelled {
		t.Fatalf("status = %s", order.Status())
	}

	// CANCELLED 是终态
	if err := order.MarkAsPaid("p"); err == nil {
		t.Fatal("expected INVALID_TRANSITION for cancelled order pay")
	}
}

func TestStateMachine(t *testing.T) {
	cases := []struct {
		from  OrderStatus
		to    OrderStatus
		allow bool
	}{
		{StatusCreated, StatusPaid, true},
		{StatusCreated, StatusCancelled, true},
		{StatusCreated, StatusConfirmed, false},
		{StatusCreated, StatusShipped, false},
		{StatusPaid, StatusConfirmed, true},
		{StatusPaid, StatusCancelled, true},
		{StatusPaid, StatusShipped, false},
		{StatusConfirmed, StatusShipped, true},
		{StatusConfirmed, StatusCancelled, false},
		{StatusShipped, StatusCancelled, false},
		{StatusCancelled, StatusPaid, false},
	}
	for _, c := range cases {
		if got := c.from.CanTransitionTo(c.to); got != c.allow {
			t.Errorf("%s -> %s = %v, want %v", c.from, c.to, got, c.allow)
		}
	}
}

func TestConfirm(t *testing.T) {
	order, _ := Create("c-1", []*OrderItem{item(t, "p-1", 1, "10")})
	if err := order.Confirm(); err == nil {
		t.Fatal("expected error: CREATED cannot confirm directly")
	}
	if err := order.MarkAsPaid("payment-1"); err != nil {
		t.Fatalf("pay: %v", err)
	}
	if err := order.Confirm(); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if order.Status() != StatusConfirmed {
		t.Fatalf("status = %s", order.Status())
	}
}
