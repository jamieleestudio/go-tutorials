package domain

import (
	"testing"

	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared"
	"github.com/shopspring/decimal"
)

func mustDecimal(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	if err != nil {
		t.Fatalf("bad decimal %q: %v", s, err)
	}
	return d
}

func TestCreateValidatesFields(t *testing.T) {
	if _, err := Create("", mustDecimal(t, "10")); err == nil {
		t.Fatal("expected error for empty orderId")
	}
	if _, err := Create("o-1", mustDecimal(t, "0")); err == nil {
		t.Fatal("expected error for zero amount")
	}
	p, err := Create("o-1", mustDecimal(t, "99.90"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.Status() != StatusPending {
		t.Fatalf("status = %s, want PENDING", p.Status())
	}
}

func TestStateMachine(t *testing.T) {
	p, _ := Create("o-1", mustDecimal(t, "99.90"))

	// PENDING → SUCCESS
	if err := p.Succeed(); err != nil {
		t.Fatalf("succeed: %v", err)
	}
	// SUCCESS → REFUNDED
	if err := p.Refund(); err != nil {
		t.Fatalf("refund: %v", err)
	}
	// REFUNDED 是终态：不能再退
	err := p.Refund()
	if err == nil {
		t.Fatal("expected INVALID_TRANSITION for double refund")
	}
	var domErr *shared.DomainError
	if !asDomainError(err, &domErr) || domErr.Code != "INVALID_TRANSITION" {
		t.Fatalf("err = %v, want INVALID_TRANSITION", err)
	}
}

func TestRefundRequiresSuccess(t *testing.T) {
	p, _ := Create("o-1", mustDecimal(t, "99.90"))
	if err := p.Refund(); err == nil {
		t.Fatal("expected INVALID_TRANSITION for refund of PENDING payment")
	}
}

func TestPayAmountIsPositive(t *testing.T) {
	p, _ := Create("o-1", mustDecimal(t, "10"))
	if err := p.Fail("gateway timeout"); err != nil {
		t.Fatalf("fail: %v", err)
	}
	if p.Status() != StatusFailed || p.FailureReason() != "gateway timeout" {
		t.Fatalf("status=%s reason=%s", p.Status(), p.FailureReason())
	}
}

func asDomainError(err error, target **shared.DomainError) bool {
	if e, ok := err.(*shared.DomainError); ok {
		*target = e
		return true
	}
	return false
}
