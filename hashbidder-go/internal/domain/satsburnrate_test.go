package domain

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestSatsBurnRate_Zero(t *testing.T) {
	r := ZeroBurnRate()
	if !r.Amount.IsZero() {
		t.Fatal("expected zero amount")
	}
}

func TestSatsBurnRate_Invalid(t *testing.T) {
	if _, err := NewSatsBurnRate(decimal.NewFromInt(-1), time.Hour); err == nil {
		t.Fatal("expected error")
	}
	if _, err := NewSatsBurnRate(decimal.NewFromInt(1), 0); err == nil {
		t.Fatal("expected error")
	}
}

func TestSatsBurnRate_To(t *testing.T) {
	r, err := NewSatsBurnRate(decimal.NewFromInt(2400), 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	h := r.To(time.Hour)
	if h.Amount.Sub(decimal.NewFromInt(100)).Abs().GreaterThan(decimal.RequireFromString("1e-12")) || h.Period != time.Hour {
		t.Fatalf("hourly=%+v", h)
	}
	rt := r.To(time.Hour).To(24 * time.Hour).Amount
	if rt.Sub(decimal.NewFromInt(2400)).Abs().GreaterThan(decimal.RequireFromString("1e-9")) {
		t.Fatalf("roundtrip got %s", rt)
	}
}

func TestSatsBurnRate_Add(t *testing.T) {
	a, _ := NewSatsBurnRate(decimal.NewFromInt(100), 24*time.Hour)
	b, _ := NewSatsBurnRate(decimal.NewFromInt(250), 24*time.Hour)
	s := a.Add(b)
	if !s.Amount.Equal(decimal.NewFromInt(350)) || s.Period != 24*time.Hour {
		t.Fatalf("sum=%+v", s)
	}
	a2, _ := NewSatsBurnRate(decimal.NewFromInt(24), 24*time.Hour)
	b2, _ := NewSatsBurnRate(decimal.NewFromInt(1), time.Hour)
	s2 := a2.Add(b2)
	if !s2.Amount.Equal(decimal.NewFromInt(48)) {
		t.Fatalf("got %s", s2.Amount)
	}
}

func TestSatsBurnRate_Runway(t *testing.T) {
	z := ZeroBurnRate()
	if z.Runway(1_000_000) != time.Duration(1<<63-1) {
		t.Fatalf("zero rate runway")
	}
	r, _ := NewSatsBurnRate(decimal.NewFromInt(2400), 24*time.Hour)
	if d := r.Runway(500); d < 4*time.Hour+59*time.Minute || d > 5*time.Hour+1*time.Minute {
		t.Fatalf("runway=%s", d)
	}
	if d := r.Runway(0); d != 0 {
		t.Fatalf("runway=%s", d)
	}
}
