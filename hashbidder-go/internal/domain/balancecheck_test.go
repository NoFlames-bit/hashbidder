package domain_test

import (
	"testing"
	"time"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/testhelpers"

	"github.com/shopspring/decimal"
)

const (
	burnRateSatPerDay = 2_500
)

func TestCheckBalance_NoCreates(t *testing.T) {
	plan := domain.PlanBidChanges(testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool), nil)
	res := domain.CheckBalance(plan, 0)
	if res.Status != domain.BalanceSufficient || res.RequiredSat != 0 {
		t.Fatalf("bad result: %+v", res)
	}
	if !res.BurnRate.Amount.IsZero() || !res.RunwayInfinite {
		t.Fatalf("burn/runway: %+v infinite=%v", res.BurnRate, res.RunwayInfinite)
	}
}

func TestCheckBalance_SufficientLongRunway(t *testing.T) {
	plan := domain.PlanBidChanges(testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool, testhelpers.MakeBidConfig(500, "5.0")), nil)
	avail := domain.Sats(110_000)
	res := domain.CheckBalance(plan, avail)
	if res.Status != domain.BalanceSufficient || res.RequiredSat != 100_000 {
		t.Fatalf("bad: %+v", res)
	}
	if !res.BurnRate.Amount.Equal(decimal.NewFromInt(burnRateSatPerDay)) {
		t.Fatalf("burn=%s", res.BurnRate.Amount)
	}
	// 110k sats at 2.5k sat/day is ~44 days.
	if got := res.Runway; got < 43*24*time.Hour || got > 45*24*time.Hour {
		t.Fatalf("runway=%s", got)
	}
}

func TestCheckBalance_ShortRunwayLow(t *testing.T) {
	plan := domain.PlanBidChanges(testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool, testhelpers.MakeBidConfig(20_000, "10.0")), nil)
	avail := domain.Sats(100_000)
	res := domain.CheckBalance(plan, avail)
	if res.Status != domain.BalanceLow {
		t.Fatalf("status=%s", res.Status)
	}
	if res.Runway < 11*time.Hour || res.Runway > 13*time.Hour {
		t.Fatalf("runway=%s", res.Runway)
	}
}

func TestCheckBalance_ExactlyThresholdSufficient(t *testing.T) {
	plan := domain.PlanBidChanges(testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool, testhelpers.MakeBidConfig(3_333, "10.0")), nil)
	avail := domain.Sats(100_000)
	res := domain.CheckBalance(plan, avail)
	if res.Status != domain.BalanceSufficient {
		t.Fatalf("status=%s", res.Status)
	}
	if res.Runway < domain.LowBalanceRunway-1*time.Hour || res.Runway > domain.LowBalanceRunway+1*time.Hour {
		t.Fatalf("runway=%s want~%s", res.Runway, domain.LowBalanceRunway)
	}
}

func TestCheckBalance_Insufficient(t *testing.T) {
	plan := domain.PlanBidChanges(testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool, testhelpers.MakeBidConfig(500, "5.0")), nil)
	res := domain.CheckBalance(plan, 50_000)
	if res.Status != domain.BalanceInsufficient || res.RequiredSat != 100_000 || res.AvailableSat != 50_000 {
		t.Fatalf("bad: %+v", res)
	}
}

func TestCheckBalance_InsufficientPrecedence(t *testing.T) {
	plan := domain.PlanBidChanges(testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool,
		testhelpers.MakeBidConfig(500, "5.0"),
		testhelpers.MakeBidConfig(500, "5.0"),
	), nil)
	res := domain.CheckBalance(plan, 100_000)
	if res.Status != domain.BalanceInsufficient || res.RequiredSat != 200_000 {
		t.Fatalf("bad: %+v", res)
	}
}

func TestCheckBalance_BurnRateSums(t *testing.T) {
	plan := domain.PlanBidChanges(testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool,
		testhelpers.MakeBidConfig(500, "5.0"),
		testhelpers.MakeBidConfig(500, "5.0"),
	), nil)
	res := domain.CheckBalance(plan, domain.Sats(250_000))
	if !res.BurnRate.Amount.Equal(decimal.NewFromInt(burnRateSatPerDay * 2)) {
		t.Fatalf("burn=%s", res.BurnRate.Amount)
	}
	if res.RequiredSat != 200_000 {
		t.Fatalf("required=%d", res.RequiredSat)
	}
}
