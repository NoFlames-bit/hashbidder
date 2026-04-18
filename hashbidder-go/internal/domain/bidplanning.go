package domain

import (
	"sort"
	"strings"
)

type CancelReason string

const (
	CancelReasonUnmatched        CancelReason = "no matching config entry"
	CancelReasonUpstreamMismatch CancelReason = "upstream mismatch (cannot edit upstream)"
)

type EditAction struct {
	Bid             UserBid
	NewPrice        HashratePrice
	NewSpeedLimitPH Hashrate
	OldPrice        HashratePrice
	OldSpeedLimitPH Hashrate
}

func (e EditAction) PriceChanged() bool {
	return e.OldPrice.To(PH, Day).Sats != e.NewPrice.To(PH, Day).Sats
}

func (e EditAction) SpeedLimitChanged() bool {
	return e.OldSpeedLimitPH.Cmp(e.NewSpeedLimitPH) != 0
}

type CreateAction struct {
	Config   BidConfig
	Amount   Sats
	Upstream Upstream
	Replaces *UserBid
}

type CancelAction struct {
	Bid    UserBid
	Reason CancelReason
}

type UnchangedBid struct {
	Bid UserBid
}

// DeferredCreate is a config row that would become a new bid, but an existing
// order already uses the same delivery slot while not ACTIVE/CREATED. We skip
// creating a duplicate until a later run when the order is manageable again.
type DeferredCreate struct {
	Config      BidConfig
	Amount      Sats
	Upstream    Upstream
	BlockingBid UserBid
}

type ReconciliationPlan struct {
	Edits           []EditAction
	Creates         []CreateAction
	Cancels         []CancelAction
	Unchanged       []UnchangedBid
	DeferredCreates []DeferredCreate
}

func fieldDiffCount(bid UserBid, cfg BidConfig) int {
	diffs := 0
	if bid.Price.To(PH, Day).Sats != cfg.Price.To(PH, Day).Sats {
		diffs++
	}
	if bid.SpeedLimitPH.Cmp(cfg.SpeedLimit) != 0 {
		diffs++
	}
	return diffs
}

func upstreamEqual(a, b *Upstream) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if strings.TrimSpace(a.Identity) != strings.TrimSpace(b.Identity) {
		return false
	}
	return SameStratumURL(*a, *b)
}

// SameStratumURL reports whether two upstreams use the same stratum endpoint
// (host + port). Scheme is ignored so stratum+tcp and stratum+ssl match, which
// matters when the config and API disagree on TLS naming for the same pool.
func SameStratumURL(a, b Upstream) bool {
	return strings.EqualFold(strings.TrimSpace(a.URL.Host()), strings.TrimSpace(b.URL.Host())) &&
		strings.TrimSpace(a.URL.Port()) == strings.TrimSpace(b.URL.Port())
}

// managedIdentities returns worker names (full identity strings) that appear in
// explicit [[bids]] rows. Used so we do not cancel sibling orders on the same
// stratum URL when the config only manages a subset of workers.
func managedIdentities(cfg SetBidsConfig) map[string]struct{} {
	m := make(map[string]struct{}, len(cfg.Bids))
	for _, entry := range cfg.Bids {
		u := EffectiveUpstream(cfg, entry)
		m[strings.TrimSpace(u.Identity)] = struct{}{}
	}
	return m
}

// EffectiveUpstream returns the delivery destination for a config row.
func EffectiveUpstream(cfg SetBidsConfig, entry BidConfig) Upstream {
	if strings.TrimSpace(entry.Identity) != "" {
		return Upstream{URL: cfg.Upstream.URL, Identity: strings.TrimSpace(entry.Identity)}
	}
	return Upstream{URL: cfg.Upstream.URL, Identity: strings.TrimSpace(cfg.Upstream.Identity)}
}

func remainingSortKey(b UserBid) int64 {
	if b.AmountRemainingSat != nil {
		return int64(*b.AmountRemainingSat)
	}
	return int64(b.AmountSat)
}

// bidBetterTie returns true if a should win over b when fieldDiffCount is equal.
func bidBetterTie(a, b UserBid) bool {
	ra, rb := remainingSortKey(a), remainingSortKey(b)
	if ra != rb {
		return ra > rb
	}
	return a.ID < b.ID
}

func blockingNonManageableBidForSlot(all []UserBid, slotUp Upstream) *UserBid {
	for i := range all {
		b := &all[i]
		if _, ok := ManageableStatuses[b.Status]; ok {
			continue
		}
		if IsTerminalBidOrderStatus(b.Status) {
			continue
		}
		if b.Upstream == nil || !upstreamEqual(b.Upstream, &slotUp) {
			continue
		}
		return b
	}
	return nil
}

func removeBidByID(bids []UserBid, id BidID) []UserBid {
	for i, b := range bids {
		if b.ID == id {
			return append(bids[:i], bids[i+1:]...)
		}
	}
	return bids
}

// PlanBidChanges matches each config row to at most one manageable bid with the
// same effective upstream (URL + identity), preferring fewest price/speed diffs
// then higher remaining collateral. Unmatched slots become creates, unless a
// non-terminal bid already occupies the slot (e.g. PAUSED) — then the create is
// deferred (DeferredCreates) to avoid a duplicate order.
//
// Orphan bids: if len(cfg.Bids)==0, every remaining manageable bid is canceled.
// Otherwise a remaining bid is canceled only when it is on a different stratum
// URL than [upstream], or its identity is listed in the config (surplus duplicate
// for a managed worker). Bids on the same URL as [upstream] whose identity is
// not listed in any [[bids]] row are left alone so you can add workers incrementally.
func PlanBidChanges(cfg SetBidsConfig, current []UserBid) ReconciliationPlan {
	manageable := make([]UserBid, 0, len(current))
	for _, b := range current {
		if _, ok := ManageableStatuses[b.Status]; ok {
			manageable = append(manageable, b)
		}
	}
	sort.SliceStable(manageable, func(i, j int) bool {
		return remainingSortKey(manageable[i]) > remainingSortKey(manageable[j])
	})

	available := append([]UserBid(nil), manageable...)

	edits := []EditAction{}
	creates := []CreateAction{}
	cancels := []CancelAction{}
	unchanged := []UnchangedBid{}
	deferred := []DeferredCreate{}

	for _, entry := range cfg.Bids {
		slotUp := EffectiveUpstream(cfg, entry)
		var best *UserBid
		bestScore := 0
		first := true
		for i := range available {
			bid := &available[i]
			if bid.Upstream == nil || !upstreamEqual(bid.Upstream, &slotUp) {
				continue
			}
			sc := fieldDiffCount(*bid, entry)
			if first || sc < bestScore || (sc == bestScore && bidBetterTie(*bid, *best)) {
				best = bid
				bestScore = sc
				first = false
			}
		}
		if best == nil {
			if blocker := blockingNonManageableBidForSlot(current, slotUp); blocker != nil {
				deferred = append(deferred, DeferredCreate{
					Config:      entry,
					Amount:      cfg.DefaultAmount,
					Upstream:    slotUp,
					BlockingBid: *blocker,
				})
				continue
			}
			creates = append(creates, CreateAction{
				Config:   entry,
				Amount:   cfg.DefaultAmount,
				Upstream: slotUp,
				Replaces: nil,
			})
			continue
		}
		bid := *best
		available = removeBidByID(available, bid.ID)
		if fieldDiffCount(bid, entry) == 0 {
			unchanged = append(unchanged, UnchangedBid{Bid: bid})
			continue
		}
		edits = append(edits, EditAction{
			Bid:             bid,
			NewPrice:        entry.Price,
			NewSpeedLimitPH: entry.SpeedLimit,
			OldPrice:        bid.Price,
			OldSpeedLimitPH: bid.SpeedLimitPH,
		})
	}

	if len(cfg.Bids) == 0 {
		for _, bid := range available {
			cancels = append(cancels, CancelAction{Bid: bid, Reason: CancelReasonUnmatched})
		}
	} else {
		managed := managedIdentities(cfg)
		for _, bid := range available {
			if bid.Upstream == nil {
				cancels = append(cancels, CancelAction{Bid: bid, Reason: CancelReasonUnmatched})
				continue
			}
			if !SameStratumURL(*bid.Upstream, cfg.Upstream) {
				cancels = append(cancels, CancelAction{Bid: bid, Reason: CancelReasonUnmatched})
				continue
			}
			if _, ok := managed[strings.TrimSpace(bid.Upstream.Identity)]; ok {
				cancels = append(cancels, CancelAction{Bid: bid, Reason: CancelReasonUnmatched})
				continue
			}
		}
	}

	return ReconciliationPlan{
		Edits:           edits,
		Creates:         creates,
		Cancels:         cancels,
		Unchanged:       unchanged,
		DeferredCreates: deferred,
	}
}
