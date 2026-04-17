package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestHashrate_Zero(t *testing.T) {
	h, err := NewHashrate(decimal.Zero, PH, Second)
	if err != nil {
		t.Fatal(err)
	}
	if !h.Value.IsZero() {
		t.Fatal("expected zero")
	}
}

func TestHashrate_String(t *testing.T) {
	h, _ := NewHashrate(decimal.NewFromInt(5), EH, Day)
	if h.String() != "5 EH/Day" {
		t.Fatalf("got %q", h.String())
	}
}

func TestHashrate_NegativeRejected(t *testing.T) {
	if _, err := NewHashrate(decimal.NewFromInt(-1), PH, Second); err == nil {
		t.Fatal("expected error")
	}
}

func TestHashrate_Conversion(t *testing.T) {
	h, _ := NewHashrate(decimal.NewFromInt(10), PH, Second)
	if !h.To(PH, Second).Value.Equal(decimal.NewFromInt(10)) {
		t.Fatal("identity")
	}
	onePHs, _ := NewHashrate(decimal.NewFromInt(1), PH, Second)
	conv := onePHs.To(EH, Day)
	if conv.HashUnit != EH || conv.TimeUnit != Day || !conv.Value.Equal(decimal.RequireFromString("86.4")) {
		t.Fatalf("got %s %s/%v", conv.Value, conv.HashUnit.Name(), conv.TimeUnit)
	}
	back := conv.To(PH, Second)
	if !back.Value.Equal(decimal.NewFromInt(1)) {
		t.Fatalf("back=%s", back.Value)
	}
}

func TestHashrate_Arithmetic(t *testing.T) {
	a, _ := NewHashrate(decimal.NewFromInt(3), PH, Second)
	b, _ := NewHashrate(decimal.NewFromInt(2), PH, Second)
	if !a.Add(b).Value.Equal(decimal.NewFromInt(5)) {
		t.Fatal("add same units")
	}
	ehd, _ := NewHashrate(decimal.NewFromInt(1), EH, Day)
	onePHs := ehd.To(PH, Second)
	sum := ehd.Add(onePHs)
	if sum.HashUnit != EH || sum.TimeUnit != Day {
		t.Fatalf("mixed units got %+v", sum)
	}
	diff := sum.Value.Sub(decimal.NewFromInt(2)).Abs()
	if diff.GreaterThan(HashrateTolerance) {
		t.Fatalf("mixed add=%s", sum.Value)
	}
	x, _ := NewHashrate(decimal.NewFromInt(5), TH, Second)
	y, _ := NewHashrate(decimal.NewFromInt(3), TH, Second)
	if !x.Sub(y).Value.Equal(decimal.NewFromInt(2)) {
		t.Fatal("sub")
	}
	z, _ := NewHashrate(decimal.NewFromInt(1), PH, Second)
	if !z.Sub(z).Value.IsZero() {
		t.Fatal("self sub")
	}
}
