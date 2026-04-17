package domain

import (
	"sort"
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

type ReconciliationPlan struct {
	Edits     []EditAction
	Creates   []CreateAction
	Cancels   []CancelAction
	Unchanged []UnchangedBid
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
	as, ah, ap := a.URL.Key()
	bs, bh, bp := b.URL.Key()
	return a.Identity == b.Identity && as == bs && ah == bh && ap == bp
}

func PlanBidChanges(cfg SetBidsConfig, current []UserBid) ReconciliationPlan {
	manageable := make([]UserBid, 0, len(current))
	for _, b := range current {
		if _, ok := ManageableStatuses[b.Status]; ok {
			manageable = append(manageable, b)
		}
	}
	sort.SliceStable(manageable, func(i, j int) bool {
		ri := manageable[i].AmountRemainingSat
		rj := manageable[j].AmountRemainingSat
		var vi, vj int64
		if ri != nil {
			vi = int64(*ri)
		} else {
			vi = int64(manageable[i].AmountSat)
		}
		if rj != nil {
			vj = int64(*rj)
		} else {
			vj = int64(manageable[j].AmountSat)
		}
		return vi > vj
	})

	unmatched := make([]int, 0, len(cfg.Bids))
	for i := range cfg.Bids {
		unmatched = append(unmatched, i)
	}

	edits := []EditAction{}
	creates := []CreateAction{}
	cancels := []CancelAction{}
	unchanged := []UnchangedBid{}

	paired := map[BidID]int{}

	for _, bid := range manageable {
		if len(unmatched) == 0 {
			break
		}
		bestIdx := unmatched[0]
		bestScore := fieldDiffCount(bid, cfg.Bids[bestIdx])
		for _, ci := range unmatched[1:] {
			sc := fieldDiffCount(bid, cfg.Bids[ci])
			if sc < bestScore {
				bestScore = sc
				bestIdx = ci
			}
		}
		paired[bid.ID] = bestIdx
		// remove bestIdx from unmatched
		for i, v := range unmatched {
			if v == bestIdx {
				unmatched = append(unmatched[:i], unmatched[i+1:]...)
				break
			}
		}
	}

	for _, bid := range manageable {
		ci, ok := paired[bid.ID]
		if !ok {
			cancels = append(cancels, CancelAction{Bid: bid, Reason: CancelReasonUnmatched})
			continue
		}
		entry := cfg.Bids[ci]
		diffs := fieldDiffCount(bid, entry)
		if !upstreamEqual(bid.Upstream, &cfg.Upstream) {
			bcopy := bid
			cancels = append(cancels, CancelAction{Bid: bid, Reason: CancelReasonUpstreamMismatch})
			creates = append(creates, CreateAction{
				Config:   entry,
				Amount:   cfg.DefaultAmount,
				Upstream: cfg.Upstream,
				Replaces: &bcopy,
			})
			continue
		}
		if diffs == 0 {
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

	for _, ci := range unmatched {
		creates = append(creates, CreateAction{
			Config:   cfg.Bids[ci],
			Amount:   cfg.DefaultAmount,
			Upstream: cfg.Upstream,
			Replaces: nil,
		})
	}

	return ReconciliationPlan{
		Edits:     edits,
		Creates:   creates,
		Cancels:   cancels,
		Unchanged: unchanged,
	}
}
