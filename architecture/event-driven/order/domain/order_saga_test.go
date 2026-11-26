package domain

import (
	"errors"
	"testing"

	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared"
	"github.com/shopspring/decimal"
)

// OrderSagaTest 的 Go 对照 —— 7 个用例逐一对应（Saga 状态机是 P5 的灵魂）。

func newSampleOrder(t *testing.T) *Order {
	t.Helper()
	item, err := NewOrderItem("p1", "Laptop", 1, decimal.RequireFromString("100"))
	if err != nil {
		t.Fatalf("NewOrderItem: %v", err)
	}
	order, err := Create("c1", []*OrderItem{item})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return order
}

func TestOrderStartsCreatedWithNoSagaProgress(t *testing.T) {
	order := newSampleOrder(t)
	if order.Status() != StatusCreated {
		t.Fatalf("status = %s, want CREATED", order.Status())
	}
	if order.PaymentSucceeded() || order.InventoryDeducted() {
		t.Fatal("fresh order must have no saga progress")
	}
}

func TestSingleSagaStepDoesNotConfirm(t *testing.T) {
	order := newSampleOrder(t)
	if err := order.MarkPaymentSucceeded("pay-1"); err != nil {
		t.Fatalf("MarkPaymentSucceeded: %v", err)
	}
	if order.Status() != StatusCreated {
		t.Fatalf("payment alone: status = %s, want CREATED", order.Status())
	}

	order2 := newSampleOrder(t)
	if err := order2.MarkInventoryDeducted(); err != nil {
		t.Fatalf("MarkInventoryDeducted: %v", err)
	}
	if order2.Status() != StatusCreated {
		t.Fatalf("inventory alone: status = %s, want CREATED", order2.Status())
	}
}

func TestBothSagaStepsConfirmTheOrder(t *testing.T) {
	order := newSampleOrder(t)
	_ = order.MarkPaymentSucceeded("pay-1")
	_ = order.MarkInventoryDeducted()
	if order.Status() != StatusConfirmed {
		t.Fatalf("status = %s, want CONFIRMED", order.Status())
	}
	if order.PaymentID() != "pay-1" {
		t.Fatalf("paymentID = %s, want pay-1", order.PaymentID())
	}
}

func TestInventoryAfterPaymentAlsoConfirms(t *testing.T) {
	order := newSampleOrder(t)
	_ = order.MarkInventoryDeducted()
	_ = order.MarkPaymentSucceeded("pay-1")
	if order.Status() != StatusConfirmed {
		t.Fatalf("status = %s, want CONFIRMED (order of arrivals must not matter)", order.Status())
	}
}

func TestCancelFromCreatedAllowed(t *testing.T) {
	order := newSampleOrder(t)
	if err := order.Cancel("payment failed"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if order.Status() != StatusCancelled {
		t.Fatalf("status = %s, want CANCELLED", order.Status())
	}
}

func TestCannotCancelConfirmedOrder(t *testing.T) {
	order := newSampleOrder(t)
	_ = order.MarkPaymentSucceeded("pay-1")
	_ = order.MarkInventoryDeducted()
	err := order.Cancel("too late")
	if err == nil {
		t.Fatal("cancelling CONFIRMED order must fail")
	}
	var ruleErr *shared.DomainError
	if !errors.As(err, &ruleErr) {
		t.Fatalf("want BusinessRuleViolation, got %T", err)
	}
}

func TestCannotRecordPaymentTwice(t *testing.T) {
	order := newSampleOrder(t)
	_ = order.MarkPaymentSucceeded("pay-1")
	_ = order.MarkInventoryDeducted() // now CONFIRMED
	err := order.MarkPaymentSucceeded("pay-2")
	if err == nil {
		t.Fatal("recording payment on CONFIRMED order must fail")
	}
	var ruleErr *shared.DomainError
	if !errors.As(err, &ruleErr) {
		t.Fatalf("want BusinessRuleViolation, got %T", err)
	}
}
