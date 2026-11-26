package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

// 领域单测：无任何框架/仓储/网络依赖（对应 Java 版的领域层测试风格）。

func mustDecimal(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	if err != nil {
		t.Fatalf("bad decimal %q: %v", s, err)
	}
	return d
}

func TestCreateValidatesFields(t *testing.T) {
	if _, err := Create("", mustDecimal(t, "1"), 1); err == nil {
		t.Fatal("expected error for empty name")
	}
	if _, err := Create("P", mustDecimal(t, "0"), 1); err == nil {
		t.Fatal("expected error for zero price")
	}
	if _, err := Create("P", mustDecimal(t, "1"), -1); err == nil {
		t.Fatal("expected error for negative stock")
	}
}

func TestDeductStock(t *testing.T) {
	p, err := Create("Keyboard", mustDecimal(t, "19.90"), 5)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := p.DeductStock(3); err != nil {
		t.Fatalf("deduct: %v", err)
	}
	if p.Stock() != 2 {
		t.Fatalf("stock = %d, want 2", p.Stock())
	}
	if err := p.DeductStock(3); err == nil {
		t.Fatal("expected OUT_OF_STOCK")
	} else if err.Error()[:12] != "OUT_OF_STOCK" {
		t.Fatalf("error = %v, want OUT_OF_STOCK", err)
	}
}

func TestChangePrice(t *testing.T) {
	p, _ := Create("P", mustDecimal(t, "10"), 1)
	if err := p.ChangePrice(mustDecimal(t, "12.5")); err != nil {
		t.Fatalf("change price: %v", err)
	}
	if p.Price().String() != "12.5" {
		t.Fatalf("price = %s", p.Price())
	}
	if err := p.ChangePrice(mustDecimal(t, "-1")); err == nil {
		t.Fatal("expected error for negative price")
	}
}
