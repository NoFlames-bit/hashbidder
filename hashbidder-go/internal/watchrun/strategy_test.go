package watchrun

import (
	"testing"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/braiins"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/cfg"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"

	"github.com/shopspring/decimal"
)

func mustPHDayPrice(sats int64) domain.HashratePrice {
	per, _ := domain.NewHashrate(decimal.NewFromInt(1), domain.PH, domain.Day)
	p, err := domain.NewHashratePrice(domain.Sats(sats), per)
	if err != nil {
		panic(err)
	}
	return p
}

func mustTick(sats int64) domain.PriceTick {
	tk, err := domain.NewPriceTick(domain.Sats(sats))
	if err != nil {
		panic(err)
	}
	return tk
}

func TestStepTowardEH_increase(t *testing.T) {
	tick := mustTick(1000)
	live, err := ehDayFromWire(46_350_000)
	if err != nil {
		t.Fatal(err)
	}
	target, err := ehDayFromWire(46_352_000)
	if err != nil {
		t.Fatal(err)
	}
	got, err := stepTowardEH(live, target, tick, 1, true)
	if err != nil {
		t.Fatal(err)
	}
	if wireSats(got) != 46_351_000 {
		t.Fatalf("got %d", wireSats(got))
	}
}

func TestStepTowardEH_respectsAllowDown(t *testing.T) {
	tick := mustTick(1000)
	live, err := ehDayFromWire(46_352_000)
	if err != nil {
		t.Fatal(err)
	}
	target, err := ehDayFromWire(46_350_000)
	if err != nil {
		t.Fatal(err)
	}
	got, err := stepTowardEH(live, target, tick, 5, false)
	if err != nil {
		t.Fatal(err)
	}
	if wireSats(got) != wireSats(live) {
		t.Fatalf("expected no decrease, got wire=%d", wireSats(got))
	}
}

func TestApplyServedFloorBand_noServedBook(t *testing.T) {
	rule := cfg.BidWatchRule{
		Strategy:        cfg.StrategyServedFloorBand,
		MinPrice:        mustPHDayPrice(400_000),
		MaxPrice:        mustPHDayPrice(500_000),
		MaxTicksPerStep: 1,
	}
	tick := mustTick(1000)
	live := mustPHDayPrice(450_000)
	book := braiins.OrderBook{Bids: nil}
	np, changed, err := ApplyServedFloorBand(rule, tick, book, live, false)
	if err != nil {
		t.Fatal(err)
	}
	if changed || np.To(domain.PH, domain.Day).Sats != live.To(domain.PH, domain.Day).Sats {
		t.Fatalf("expected noop %+v changed=%v", np, changed)
	}
}

func obBid(priceSat int64, matchedPH string) braiins.BidItem {
	hm, err := domain.NewHashrate(decimal.RequireFromString(matchedPH), domain.PH, domain.Second)
	if err != nil {
		panic(err)
	}
	sl, _ := domain.NewHashrate(decimal.NewFromInt(10), domain.PH, domain.Second)
	per, _ := domain.NewHashrate(decimal.NewFromInt(1), domain.EH, domain.Day)
	pr, err := domain.NewHashratePrice(domain.Sats(priceSat), per)
	if err != nil {
		panic(err)
	}
	return braiins.BidItem{Price: pr, HrMatchedPH: hm, SpeedLimitPH: sl}
}

func ehDayPriceSat(sats int64) domain.HashratePrice {
	per, _ := domain.NewHashrate(decimal.NewFromInt(1), domain.EH, domain.Day)
	p, err := domain.NewHashratePrice(domain.Sats(sats), per)
	if err != nil {
		panic(err)
	}
	return p
}

func TestApplyServedDepthBand_skipsThinCheapest(t *testing.T) {
	floor, _ := domain.NewHashrate(decimal.NewFromInt(4), domain.PH, domain.Second)
	rule := cfg.BidWatchRule{
		Strategy:             cfg.StrategyServedDepthBand,
		MinPrice:             ehDayPriceSat(400_000),
		MaxPrice:             ehDayPriceSat(1_000_000),
		MaxTicksPerStep:      10,
		ServedLiquidityFloor: floor,
	}
	tick := mustTick(1000)
	live := ehDayPriceSat(750_000)
	book := braiins.OrderBook{Bids: []braiins.BidItem{
		obBid(700_000, "2"),
		obBid(800_000, "3"),
		obBid(900_000, "1"),
	}}
	np, changed, err := ApplyServedDepthBand(rule, tick, book, live, false)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected price move")
	}
	// stepTowardEH is capped by max_ticks_per_step (10) on the EH/day wire grid (tick 1000).
	if want := int64(750_000 + 10*1000); int64(np.Sats) != want {
		t.Fatalf("got sats %d want %d", np.Sats, want)
	}
}
