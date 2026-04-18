package targethr

import (
	"testing"
	"time"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/braiins"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/testutil"

	"github.com/shopspring/decimal"
)

func phS(s string) domain.Hashrate {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	h, err := domain.NewHashrate(d, domain.PH, domain.Second)
	if err != nil {
		panic(err)
	}
	return h
}

func TestComputeNeededHashrate(t *testing.T) {
	cases := []struct {
		name   string
		target string
		cur    string
		want   string
	}{
		{"below", "10", "5", "15"},
		{"at", "10", "10", "10"},
		{"modestly_above", "12", "15", "9"},
		{"clamp", "10", "25", "0"},
	}
	for _, tc := range cases {
		got := ComputeNeededHashrate(phS(tc.target), phS(tc.cur))
		if !got.Value.Equal(decimal.RequireFromString(tc.want)) || got.HashUnit != domain.PH || got.TimeUnit != domain.Second {
			t.Fatalf("%s: got %s", tc.name, got.Value)
		}
	}
}

func TestComputeNeededHashrate_UnitNormalization(t *testing.T) {
	target, _ := domain.NewHashrate(decimal.NewFromInt(864), domain.PH, domain.Day)
	current, _ := domain.NewHashrate(decimal.Zero, domain.PH, domain.Second)
	got := ComputeNeededHashrate(target, current)
	if got.HashUnit != domain.PH || got.TimeUnit != domain.Second {
		t.Fatalf("units: %+v", got)
	}
}

func TestDistributeBids(t *testing.T) {
	t.Run("five_three", func(t *testing.T) {
		s := domain.DistributeBids(phS("5"), 3)
		if len(s) != 3 {
			t.Fatalf("len=%d", len(s))
		}
		want := decimal.RequireFromString("1.67")
		for _, x := range s {
			if !x.Value.Equal(want) {
				t.Fatalf("got %s", x.Value)
			}
		}
	})
	t.Run("three_seven", func(t *testing.T) {
		s := domain.DistributeBids(phS("3"), 7)
		if len(s) != 3 {
			t.Fatalf("len=%d", len(s))
		}
		one := decimal.RequireFromString("1.00")
		for _, x := range s {
			if !x.Value.Equal(one) {
				t.Fatalf("got %s", x.Value)
			}
		}
	})
	t.Run("two_five_four", func(t *testing.T) {
		s := domain.DistributeBids(phS("2.5"), 4)
		if len(s) != 2 {
			t.Fatalf("len=%d", len(s))
		}
		w := decimal.RequireFromString("1.25")
		if !s[0].Value.Equal(w) || !s[1].Value.Equal(w) {
			t.Fatalf("got %#v", s)
		}
	})
	if len(domain.DistributeBids(phS("0.3"), 3)) != 0 {
		t.Fatal("expected empty for 0.3")
	}
	if len(domain.DistributeBids(phS("0"), 3)) != 0 {
		t.Fatal("expected empty for 0")
	}
	if len(domain.DistributeBids(phS("0.7"), 3)) != 1 || !domain.DistributeBids(phS("0.7"), 3)[0].Value.Equal(decimal.NewFromInt(1)) {
		t.Fatal("0.7 case")
	}
	if len(domain.DistributeBids(phS("1"), 1)) != 1 || !domain.DistributeBids(phS("1"), 1)[0].Value.Equal(decimal.RequireFromString("1.00")) {
		t.Fatal("1 ph/s max1")
	}
	s := domain.DistributeBids(phS("7"), 3)
	w233 := decimal.RequireFromString("2.33")
	for _, x := range s {
		if !x.Value.Equal(w233) {
			t.Fatalf("got %s", x.Value)
		}
	}
}

func TestDistributeBids_MaxInvalidPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	_ = domain.DistributeBids(phS("5"), 0)
}

func bidItem(priceSat int64, hrMatched string, speedLimit string) braiins.BidItem {
	hm, _ := domain.NewHashrate(decimal.RequireFromString(hrMatched), domain.PH, domain.Second)
	sl, _ := domain.NewHashrate(decimal.RequireFromString(speedLimit), domain.PH, domain.Second)
	pr, _ := domain.NewHashratePrice(domain.Sats(priceSat), mustEHDay())
	return braiins.BidItem{
		Price:        pr,
		AmountSat:    100_000,
		HrMatchedPH:  hm,
		SpeedLimitPH: sl,
	}
}

func mustEHDay() domain.Hashrate {
	h, _ := domain.NewHashrate(decimal.NewFromInt(1), domain.EH, domain.Day)
	return h
}

func mustTick100() domain.PriceTick {
	tk, err := domain.NewPriceTick(100)
	if err != nil {
		panic(err)
	}
	return tk
}

func TestFindMarketPrice(t *testing.T) {
	book := braiins.OrderBook{
		Bids: []braiins.BidItem{
			bidItem(1000, "0", "10"),
			bidItem(500, "0", "10"),
			bidItem(800, "3", "10"),
			bidItem(700, "2", "10"),
			bidItem(900, "1", "10"),
		},
	}
	pr, err := FindMarketPrice(book, mustTick100())
	if err != nil || int64(pr.Sats) != 800 {
		t.Fatalf("got %v err=%v", pr.Sats, err)
	}
}

func TestResolveCooldowns_tier1SkipsHistory(t *testing.T) {
	tick, _ := domain.NewPriceTick(1000)
	settings := braiins.MarketSettings{
		MinBidPriceDecreasePeriod:      time.Minute,
		MinBidSpeedLimitDecreasePeriod: 2 * time.Minute,
		PriceTick:                      tick,
	}
	now := time.Unix(10_000, 0).UTC()
	bid := domain.UserBid{ID: "BX", LastUpdated: now.Add(-5 * time.Minute)}
	c := testutil.NewFakeClient(testutil.WithCurrentBids(bid))
	out, err := ResolveCooldowns(c, []domain.UserBid{bid}, settings, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Cooldown.PriceCooldown || out[0].Cooldown.SpeedCooldown {
		t.Fatalf("got %+v", out[0])
	}
	for _, call := range c.Calls {
		if len(call) > 0 && call[0] == "get_bid_history" {
			t.Fatalf("unexpected history fetch: %v", c.Calls)
		}
	}
}

func TestResolveCooldowns_historyAuthoritative(t *testing.T) {
	tick, _ := domain.NewPriceTick(1000)
	settings := braiins.MarketSettings{
		MinBidPriceDecreasePeriod:      time.Hour,
		MinBidSpeedLimitDecreasePeriod: time.Hour,
		PriceTick:                      tick,
	}
	now := time.Date(2026, 4, 17, 9, 0, 0, 0, time.UTC)
	bid := domain.UserBid{ID: "H1", LastUpdated: now.Add(-time.Minute)}
	// Recent update → tier-1 does not clear; empty history → no decreases → both cooldown false.
	c := testutil.NewFakeClient(
		testutil.WithCurrentBids(bid),
		testutil.WithBidHistory("H1", domain.NewBidHistory(nil)),
	)
	out, err := ResolveCooldowns(c, []domain.UserBid{bid}, settings, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Cooldown.PriceCooldown || out[0].Cooldown.SpeedCooldown {
		t.Fatalf("got %+v", out[0].Cooldown)
	}
}

func TestResolveCooldowns_apiErrorConservativeFallback(t *testing.T) {
	tick, _ := domain.NewPriceTick(1000)
	settings := braiins.MarketSettings{
		MinBidPriceDecreasePeriod:      2 * time.Minute,
		MinBidSpeedLimitDecreasePeriod: time.Minute,
		PriceTick:                      tick,
	}
	now := time.Unix(1000, 0).UTC()
	bid := domain.UserBid{ID: "B1", LastUpdated: now.Add(-90 * time.Second)}
	errs := map[string][]*braiins.APIError{
		"get_bid_history:B1": {{StatusCode: 404, Message: "not found"}},
	}
	c := testutil.NewFakeClient(
		testutil.WithCurrentBids(bid),
		testutil.WithErrors(errs),
	)
	out, err := ResolveCooldowns(c, []domain.UserBid{bid}, settings, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("len=%d", len(out))
	}
	// price_free false → conservative true; speed_free true → conservative false.
	if !out[0].Cooldown.PriceCooldown || out[0].Cooldown.SpeedCooldown {
		t.Fatalf("got %+v", out[0].Cooldown)
	}
}
