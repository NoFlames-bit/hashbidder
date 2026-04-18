package domain

import (
	"sort"
	"time"
)

// BidHistoryEntry is one point on a bid's price/speed timeline (as returned by
// the bid detail API). Adjacent entries in a normalised [BidHistory] are used
// to detect strict decreases per field.
type BidHistoryEntry struct {
	Timestamp    time.Time
	Price        HashratePrice
	SpeedLimitPH Hashrate
}

// BidHistory holds a bid's history entries in normalised newest-first order.
type BidHistory struct {
	entries []BidHistoryEntry
}

// NewBidHistory returns a history with entries sorted by timestamp descending
// (newest first). Equal timestamps keep their relative input order (stable).
func NewBidHistory(entries []BidHistoryEntry) BidHistory {
	cp := append([]BidHistoryEntry(nil), entries...)
	sort.SliceStable(cp, func(i, j int) bool {
		ti, tj := cp[i].Timestamp, cp[j].Timestamp
		if ti.After(tj) {
			return true
		}
		if ti.Before(tj) {
			return false
		}
		return i < j
	})
	return BidHistory{entries: cp}
}

// Entries returns a defensive copy of the normalised newest-first slice.
func (h BidHistory) Entries() []BidHistoryEntry {
	return append([]BidHistoryEntry(nil), h.entries...)
}

// LastPriceDecreaseAt returns the timestamp of the most recent strict price
// decrease (newer sats strictly less than older), or nil if none.
func (h BidHistory) LastPriceDecreaseAt() *time.Time {
	for i := 0; i < len(h.entries)-1; i++ {
		newer, older := h.entries[i], h.entries[i+1]
		if newer.Price.Sats < older.Price.Sats {
			ts := newer.Timestamp
			return &ts
		}
	}
	return nil
}

// LastSpeedDecreaseAt returns the timestamp of the most recent strict speed
// decrease, or nil if none.
func (h BidHistory) LastSpeedDecreaseAt() *time.Time {
	for i := 0; i < len(h.entries)-1; i++ {
		newer, older := h.entries[i], h.entries[i+1]
		if newer.SpeedLimitPH.Less(older.SpeedLimitPH) {
			ts := newer.Timestamp
			return &ts
		}
	}
	return nil
}
