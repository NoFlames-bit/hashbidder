package domain_test

import (
	"testing"
	"time"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/testhelpers"

	"github.com/shopspring/decimal"
)

const (
	burnRateSatPerHour = 9_000_000
	burnRateSatPerDay  = burnRateSatPerHour * 24
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
	avail := domain.Sats(burnRateSatPerHour * 100)
	res := domain.CheckBalance(plan, avail)
	if res.Status != domain.BalanceSufficient || res.RequiredSat != 100_000 {
		t.Fatalf("bad: %+v", res)
	}
	if !res.BurnRate.Amount.Equal(decimal.NewFromInt(burnRateSatPerDay)) {
		t.Fatalf("burn=%s", res.BurnRate.Amount)
	}
	// Runway uses float seconds; allow small tolerance vs exact 100h.
	if got := res.Runway; got < 99*time.Hour || got > 101*time.Hour {
		t.Fatalf("runway=%s", got)
	}
}

func TestCheckBalance_ShortRunwayLow(t *testing.T) {
	plan := domain.PlanBidChanges(testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool, testhelpers.MakeBidConfig(500, "5.0")), nil)
	avail := domain.Sats(burnRateSatPerHour * 71)
	res := domain.CheckBalance(plan, avail)
	if res.Status != domain.BalanceLow {
		t.Fatalf("status=%s", res.Status)
	}
	if res.Runway < 70*time.Hour || res.Runway > 72*time.Hour {
		t.Fatalf("runway=%s", res.Runway)
	}
}

func TestCheckBalance_ExactlyThresholdSufficient(t *testing.T) {
	plan := domain.PlanBidChanges(testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool, testhelpers.MakeBidConfig(500, "5.0")), nil)
	avail := domain.Sats(burnRateSatPerHour * 72)
	res := domain.CheckBalance(plan, avail)
	if res.Status != domain.BalanceSufficient {
		t.Fatalf("status=%s", res.Status)
	}
	if res.Runway < domain.LowBalanceRunway-2*time.Hour || res.Runway > domain.LowBalanceRunway+2*time.Hour {
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
	res := domain.CheckBalance(plan, domain.Sats(burnRateSatPerHour*2*1000))
	if !res.BurnRate.Amount.Equal(decimal.NewFromInt(burnRateSatPerDay * 2)) {
		t.Fatalf("burn=%s", res.BurnRate.Amount)
	}
	if res.RequiredSat != 200_000 {
		t.Fatalf("required=%d", res.RequiredSat)
	}
}
