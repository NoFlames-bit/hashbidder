package hv

import (
	"testing"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"

	"github.com/shopspring/decimal"
)

func TestComputeHashvalue_HandComputed(t *testing.T) {
	tip, _ := domain.NewBlockHeight(840_000)
	res, err := ComputeHashvalue(decimal.NewFromInt(100_000_000_000), tip, domain.Sats(50_000_000_000))
	if err != nil {
		t.Fatal(err)
	}
	if res.Subsidy != domain.Sats(312_500_000) {
		t.Fatalf("subsidy=%d", res.Subsidy)
	}
	wantTotal := int64(2016)*int64(res.Subsidy) + 50_000_000_000
	if int64(res.TotalReward) != wantTotal {
		t.Fatalf("total reward=%d want=%d", res.TotalReward, wantTotal)
	}
	if int64(res.Hashvalue.Sats) != 67_853_502 {
		t.Fatalf("hashvalue=%d", res.Hashvalue.Sats)
	}
}

func TestComputeHashvalue_DifferentHeights(t *testing.T) {
	r1, err := ComputeHashvalue(decimal.RequireFromString("1e11"), mustHeight(210_000), 0)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := ComputeHashvalue(decimal.RequireFromString("1e11"), mustHeight(420_000), 0)
	if err != nil {
		t.Fatal(err)
	}
	if r1.Subsidy != domain.Sats(2_500_000_000) || r2.Subsidy != domain.Sats(1_250_000_000) {
		t.Fatalf("subsidy mismatch %d %d", r1.Subsidy, r2.Subsidy)
	}
	if r1.Hashvalue.Sats <= r2.Hashvalue.Sats {
		t.Fatalf("expected r1 > r2 got %d %d", r1.Hashvalue.Sats, r2.Hashvalue.Sats)
	}
}

func TestComputeHashvalue_ZeroFees(t *testing.T) {
	tip, _ := domain.NewBlockHeight(840_000)
	res, err := ComputeHashvalue(decimal.RequireFromString("1e11"), tip, 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalFees != 0 {
		t.Fatalf("fees=%d", res.TotalFees)
	}
	if int64(res.Hashvalue.Sats) <= 0 {
		t.Fatal("expected positive hashvalue")
	}
}

func TestComputeHashvalue_ZeroDifficulty(t *testing.T) {
	tip, _ := domain.NewBlockHeight(840_000)
	_, err := ComputeHashvalue(decimal.Zero, tip, 0)
	if err == nil {
		t.Fatal("expected error")
	}
}

func mustHeight(v int) domain.BlockHeight {
	h, err := domain.NewBlockHeight(v)
	if err != nil {
		panic(err)
	}
	return h
}
