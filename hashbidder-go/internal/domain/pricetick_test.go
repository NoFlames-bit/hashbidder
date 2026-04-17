package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func mustHRDec(v decimal.Decimal, hu HashUnit, tu TimeUnit) Hashrate {
	h, err := NewHashrate(v, hu, tu)
	if err != nil {
		panic(err)
	}
	return h
}

func mustPHDay(sats int64) HashratePrice {
	p, err := NewHashratePrice(Sats(sats), mustHRDec(decimal.NewFromInt(1), PH, Day))
	if err != nil {
		panic(err)
	}
	return p
}

func mustEHDay(sats int64) HashratePrice {
	p, err := NewHashratePrice(Sats(sats), mustHRDec(decimal.NewFromInt(1), EH, Day))
	if err != nil {
		panic(err)
	}
	return p
}

func mustTick(sats int64) PriceTick {
	tk, err := NewPriceTick(Sats(sats))
	if err != nil {
		panic(err)
	}
	return tk
}

func TestPriceTickConstruction(t *testing.T) {
	tk, err := NewPriceTick(Sats(1))
	if err != nil || tk.Sats != 1 {
		t.Fatalf("tick=%v err=%v", tk, err)
	}
	if _, err := NewPriceTick(0); err == nil {
		t.Fatal("expected error for zero")
	}
	if _, err := NewPriceTick(-100); err == nil {
		t.Fatal("expected error for negative")
	}
}

func TestPriceTickIsAligned(t *testing.T) {
	tick := mustTick(1000)
	if !tick.IsAligned(mustEHDay(46350000)) {
		t.Fatal("expected aligned")
	}
	if tick.IsAligned(mustEHDay(46350001)) {
		t.Fatal("expected not aligned")
	}
	if !tick.IsAligned(mustPHDay(900)) {
		t.Fatal("expected aligned across units")
	}
	if err := tick.AssertAligned(mustEHDay(46350001)); err == nil {
		t.Fatal("expected assert error")
	}
}

func TestPriceTickAlignDown(t *testing.T) {
	tick := mustTick(1000)
	if int64(tick.AlignDown(mustEHDay(46350000)).Sats) != 46350000 {
		t.Fatalf("got %d", tick.AlignDown(mustEHDay(46350000)).Sats)
	}
	if int64(tick.AlignDown(mustEHDay(46350999)).Sats) != 46350000 {
		t.Fatalf("got %d", tick.AlignDown(mustEHDay(46350999)).Sats)
	}
	d := tick.AlignDown(mustPHDay(900))
	if int64(d.Sats) != 900_000 {
		t.Fatalf("got %d", d.Sats)
	}
}

func TestPriceTickAddOne(t *testing.T) {
	tick := mustTick(1000)
	got, err := tick.AddOne(mustEHDay(46350000))
	if err != nil || int64(got.Sats) != 46351000 {
		t.Fatalf("got %v err=%v", got.Sats, err)
	}
	if _, err := tick.AddOne(mustEHDay(46350001)); err == nil {
		t.Fatal("expected error")
	}
}
