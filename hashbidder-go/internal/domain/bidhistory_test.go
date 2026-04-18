package domain

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

var bidHistT0 = time.Date(2026, 4, 17, 8, 0, 0, 0, time.UTC)

func bidHistEntry(t time.Time, priceSat int64, speed string) BidHistoryEntry {
	per, _ := NewHashrate(decimal.NewFromInt(1), EH, Day)
	pr, _ := NewHashratePrice(Sats(priceSat), per)
	spd, _ := NewHashrate(decimal.RequireFromString(speed), PH, Second)
	return BidHistoryEntry{Timestamp: t, Price: pr, SpeedLimitPH: spd}
}

func TestBidHistory_normalisationOutOfOrder(t *testing.T) {
	older := bidHistEntry(bidHistT0, 500_000, "5")
	newer := bidHistEntry(bidHistT0.Add(time.Minute), 500_000, "5")
	h := NewBidHistory([]BidHistoryEntry{older, newer})
	got := h.Entries()
	if len(got) != 2 || !got[0].Timestamp.Equal(newer.Timestamp) || !got[1].Timestamp.Equal(older.Timestamp) {
		t.Fatalf("got %+v", got)
	}
}

func TestBidHistory_normalisationShuffled(t *testing.T) {
	a := bidHistEntry(bidHistT0, 500_000, "5")
	b := bidHistEntry(bidHistT0.Add(time.Minute), 500_000, "5")
	c := bidHistEntry(bidHistT0.Add(2*time.Minute), 500_000, "5")
	h := NewBidHistory([]BidHistoryEntry{b, a, c})
	got := h.Entries()
	if len(got) != 3 || !got[0].Timestamp.Equal(c.Timestamp) || !got[1].Timestamp.Equal(b.Timestamp) || !got[2].Timestamp.Equal(a.Timestamp) {
		t.Fatalf("got %+v", got)
	}
}

func TestBidHistory_LastPriceDecreaseAt_empty(t *testing.T) {
	if NewBidHistory(nil).LastPriceDecreaseAt() != nil {
		t.Fatal("expected nil")
	}
}

func TestBidHistory_LastPriceDecreaseAt_single(t *testing.T) {
	h := NewBidHistory([]BidHistoryEntry{bidHistEntry(bidHistT0, 500_000, "5")})
	if h.LastPriceDecreaseAt() != nil {
		t.Fatal("expected nil")
	}
}

func TestBidHistory_LastPriceDecreaseAt_monotoneUp(t *testing.T) {
	h := NewBidHistory([]BidHistoryEntry{
		bidHistEntry(bidHistT0, 500_000, "5"),
		bidHistEntry(bidHistT0.Add(time.Minute), 510_000, "5"),
		bidHistEntry(bidHistT0.Add(2*time.Minute), 520_000, "5"),
	})
	if h.LastPriceDecreaseAt() != nil {
		t.Fatal("expected nil")
	}
}

func TestBidHistory_LastPriceDecreaseAt_singleDrop(t *testing.T) {
	t1 := bidHistT0.Add(time.Minute)
	h := NewBidHistory([]BidHistoryEntry{
		bidHistEntry(bidHistT0, 500_000, "5"),
		bidHistEntry(t1, 400_000, "5"),
	})
	got := h.LastPriceDecreaseAt()
	if got == nil || !got.Equal(t1) {
		t.Fatalf("got %v", got)
	}
}

func TestBidHistory_LastPriceDecreaseAt_sandwiched(t *testing.T) {
	t1 := bidHistT0.Add(time.Minute)
	t2 := bidHistT0.Add(2 * time.Minute)
	t3 := bidHistT0.Add(3 * time.Minute)
	h := NewBidHistory([]BidHistoryEntry{
		bidHistEntry(bidHistT0, 500_000, "5"),
		bidHistEntry(t1, 600_000, "5"),
		bidHistEntry(t2, 450_000, "5"),
		bidHistEntry(t3, 700_000, "5"),
	})
	got := h.LastPriceDecreaseAt()
	if got == nil || !got.Equal(t2) {
		t.Fatalf("got %v", got)
	}
}

func TestBidHistory_LastPriceDecreaseAt_noopRepeats(t *testing.T) {
	t1 := bidHistT0.Add(time.Minute)
	t2 := bidHistT0.Add(2 * time.Minute)
	t3 := bidHistT0.Add(3 * time.Minute)
	h := NewBidHistory([]BidHistoryEntry{
		bidHistEntry(bidHistT0, 500_000, "5"),
		bidHistEntry(t1, 400_000, "5"),
		bidHistEntry(t2, 400_000, "5"),
		bidHistEntry(t3, 400_000, "5"),
	})
	got := h.LastPriceDecreaseAt()
	if got == nil || !got.Equal(t1) {
		t.Fatalf("got %v", got)
	}
}

func TestBidHistory_LastPriceDecreaseAt_speedOnlyChange(t *testing.T) {
	t1 := bidHistT0.Add(time.Minute)
	h := NewBidHistory([]BidHistoryEntry{
		bidHistEntry(bidHistT0, 500_000, "10"),
		bidHistEntry(t1, 500_000, "5"),
	})
	if h.LastPriceDecreaseAt() != nil {
		t.Fatal("expected nil")
	}
}

func TestBidHistory_LastSpeedDecreaseAt_empty(t *testing.T) {
	if NewBidHistory(nil).LastSpeedDecreaseAt() != nil {
		t.Fatal("expected nil")
	}
}

func TestBidHistory_LastSpeedDecreaseAt_single(t *testing.T) {
	h := NewBidHistory([]BidHistoryEntry{bidHistEntry(bidHistT0, 500_000, "5")})
	if h.LastSpeedDecreaseAt() != nil {
		t.Fatal("expected nil")
	}
}

func TestBidHistory_LastSpeedDecreaseAt_monotoneUp(t *testing.T) {
	h := NewBidHistory([]BidHistoryEntry{
		bidHistEntry(bidHistT0, 500_000, "5"),
		bidHistEntry(bidHistT0.Add(time.Minute), 500_000, "6"),
		bidHistEntry(bidHistT0.Add(2*time.Minute), 500_000, "7"),
	})
	if h.LastSpeedDecreaseAt() != nil {
		t.Fatal("expected nil")
	}
}

func TestBidHistory_LastSpeedDecreaseAt_singleDrop(t *testing.T) {
	t1 := bidHistT0.Add(time.Minute)
	h := NewBidHistory([]BidHistoryEntry{
		bidHistEntry(bidHistT0, 500_000, "10"),
		bidHistEntry(t1, 500_000, "5"),
	})
	got := h.LastSpeedDecreaseAt()
	if got == nil || !got.Equal(t1) {
		t.Fatalf("got %v", got)
	}
}

func TestBidHistory_LastSpeedDecreaseAt_sandwiched(t *testing.T) {
	t1 := bidHistT0.Add(time.Minute)
	t2 := bidHistT0.Add(2 * time.Minute)
	t3 := bidHistT0.Add(3 * time.Minute)
	h := NewBidHistory([]BidHistoryEntry{
		bidHistEntry(bidHistT0, 500_000, "5"),
		bidHistEntry(t1, 500_000, "10"),
		bidHistEntry(t2, 500_000, "3"),
		bidHistEntry(t3, 500_000, "15"),
	})
	got := h.LastSpeedDecreaseAt()
	if got == nil || !got.Equal(t2) {
		t.Fatalf("got %v", got)
	}
}

func TestBidHistory_LastSpeedDecreaseAt_noopRepeats(t *testing.T) {
	t1 := bidHistT0.Add(time.Minute)
	t2 := bidHistT0.Add(2 * time.Minute)
	t3 := bidHistT0.Add(3 * time.Minute)
	h := NewBidHistory([]BidHistoryEntry{
		bidHistEntry(bidHistT0, 500_000, "10"),
		bidHistEntry(t1, 500_000, "5"),
		bidHistEntry(t2, 500_000, "5"),
		bidHistEntry(t3, 500_000, "5"),
	})
	got := h.LastSpeedDecreaseAt()
	if got == nil || !got.Equal(t1) {
		t.Fatalf("got %v", got)
	}
}

func TestBidHistory_LastSpeedDecreaseAt_priceOnlyChange(t *testing.T) {
	t1 := bidHistT0.Add(time.Minute)
	h := NewBidHistory([]BidHistoryEntry{
		bidHistEntry(bidHistT0, 500_000, "5"),
		bidHistEntry(t1, 400_000, "5"),
	})
	if h.LastSpeedDecreaseAt() != nil {
		t.Fatal("expected nil")
	}
}

func TestBidHistory_independentPriceAndSpeedDecreases(t *testing.T) {
	t1 := bidHistT0.Add(time.Minute)
	t2 := bidHistT0.Add(2 * time.Minute)
	t3 := bidHistT0.Add(3 * time.Minute)
	h := NewBidHistory([]BidHistoryEntry{
		bidHistEntry(bidHistT0, 500_000, "10"),
		bidHistEntry(t1, 400_000, "10"),
		bidHistEntry(t2, 400_000, "10"),
		bidHistEntry(t3, 400_000, "5"),
	})
	lp := h.LastPriceDecreaseAt()
	ls := h.LastSpeedDecreaseAt()
	if lp == nil || !lp.Equal(t1) {
		t.Fatalf("price ts got %v", lp)
	}
	if ls == nil || !ls.Equal(t3) {
		t.Fatalf("speed ts got %v", ls)
	}
}
