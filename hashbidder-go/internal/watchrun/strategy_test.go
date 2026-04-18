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
