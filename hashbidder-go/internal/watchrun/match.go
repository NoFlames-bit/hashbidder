package watchrun

import (
	"strings"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"
)

func userBidForRow(cfg domain.SetBidsConfig, entry domain.BidConfig, current []domain.UserBid) *domain.UserBid {
	slot := domain.EffectiveUpstream(cfg, entry)
	for i := range current {
		b := &current[i]
		if b.Upstream == nil {
			continue
		}
		if _, ok := domain.ManageableStatuses[b.Status]; !ok {
			continue
		}
		if strings.TrimSpace(b.Upstream.Identity) != strings.TrimSpace(slot.Identity) {
			continue
		}
		if !domain.SameStratumURL(*b.Upstream, slot) {
			continue
		}
		return b
	}
	return nil
}
