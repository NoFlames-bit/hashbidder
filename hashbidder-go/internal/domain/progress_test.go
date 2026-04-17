package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestProgress_Bounds(t *testing.T) {
	if _, err := NewProgress(decimal.Zero); err != nil {
		t.Fatal(err)
	}
	if _, err := NewProgress(decimal.NewFromInt(1)); err != nil {
		t.Fatal(err)
	}
	if _, err := NewProgress(decimal.NewFromFloat(-0.1)); err == nil {
		t.Fatal("expected error")
	}
	if _, err := NewProgress(decimal.NewFromFloat(1.1)); err == nil {
		t.Fatal("expected error")
	}
}

func TestProgress_StringAndFromPercentage(t *testing.T) {
	p, err := NewProgress(decimal.RequireFromString("0.425"))
	if err != nil {
		t.Fatal(err)
	}
	if !p.Percentage().Equal(decimal.RequireFromString("42.5")) {
		t.Fatalf("percentage got %s", p.Percentage())
	}
	p2, err := ProgressFromPercentage(decimal.RequireFromString("42.5"))
	if err != nil {
		t.Fatal(err)
	}
	if !p2.Value().Equal(decimal.RequireFromString("0.425")) {
		t.Fatalf("got %s", p2.Value())
	}
	if !p2.Percentage().Equal(decimal.RequireFromString("42.5")) {
		t.Fatalf("got %s", p2.Percentage())
	}
}

func TestProgress_Equality(t *testing.T) {
	a, _ := NewProgress(decimal.RequireFromString("0.5"))
	b, _ := NewProgress(decimal.RequireFromString("0.5"))
	c, _ := NewProgress(decimal.RequireFromString("0.51"))
	if !a.Value().Equal(b.Value()) || a.Value().Equal(c.Value()) {
		t.Fatal("equality mismatch")
	}
}
