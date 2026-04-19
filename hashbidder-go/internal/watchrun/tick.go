package watchrun

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/braiins"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/cfg"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/targethr"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/usecase"
)

// OneTick fetches market state, applies per-row strategies, and reconciles once.
func OneTick(ctx context.Context, client braiins.HashpowerClient, wm *cfg.WatchModeConfig, dryRun bool) error {
	_ = ctx
	book, err := client.GetOrderbook()
	if err != nil {
		return err
	}
	settings, err := client.GetMarketSettings()
	if err != nil {
		return err
	}
	current, err := client.GetCurrentBids()
	if err != nil {
		return err
	}
	annotated, err := targethr.ResolveCooldowns(client, current, settings, time.Now().UTC())
	if err != nil {
		return err
	}
	cdByID := make(map[domain.BidID]targethr.CooldownInfo, len(annotated))
	for _, a := range annotated {
		cdByID[a.Bid.ID] = a.Cooldown
	}

	nextBids := make([]domain.BidConfig, len(wm.SetBids.Bids))
	copy(nextBids, wm.SetBids.Bids)
	changed := false
	var priceAdjParts []string
	for i := range nextBids {
		rule := wm.Rules[i]
		if rule.Strategy == cfg.StrategyNone {
			continue
		}
		row := wm.SetBids.Bids[i]
		ub := userBidForRow(wm.SetBids, row, current)
		if ub == nil {
			continue
		}
		cd := cdByID[ub.ID]
		var (
			np   domain.HashratePrice
			ok   bool
			serr error
		)
		switch rule.Strategy {
		case cfg.StrategyServedFloorBand:
			np, ok, serr = ApplyServedFloorBand(rule, settings.PriceTick, book, ub.Price, cd.PriceCooldown)
		case cfg.StrategyServedDepthBand:
			np, ok, serr = ApplyServedDepthBand(rule, settings.PriceTick, book, ub.Price, cd.PriceCooldown)
		default:
			continue
		}
		if serr != nil {
			slog.Warn("watch strategy error", "bid", ub.ID, "err", serr)
			continue
		}
		if !ok {
			continue
		}
		if err := settings.PriceTick.AssertAligned(np); err != nil {
			slog.Warn("watch produced non-aligned price", "bid", ub.ID, "err", err)
			continue
		}
		if wireSats(np) != wireSats(nextBids[i].Price) {
			nextBids[i].Price = np
			changed = true
			liveS := int64(ub.Price.To(domain.PH, domain.Day).Sats)
			targetS := int64(np.To(domain.PH, domain.Day).Sats)
			if liveS != targetS {
				slot := domain.EffectiveUpstream(wm.SetBids, wm.SetBids.Bids[i])
				id := strings.TrimSpace(wm.SetBids.Bids[i].Identity)
				if id == "" {
					id = strings.TrimSpace(slot.Identity)
				}
				if id == "" {
					id = string(ub.ID)
				}
				priceAdjParts = append(priceAdjParts, fmt.Sprintf("%s watch_strategy=%s %d→%d sat/PH/day", id, rule.Strategy, liveS, targetS))
			}
		}
	}
	if !changed {
		slog.Debug("watch tick: no price changes")
		return nil
	}
	runCfg := domain.SetBidsConfig{
		DefaultAmount: wm.SetBids.DefaultAmount,
		Upstream:      wm.SetBids.Upstream,
		Bids:          nextBids,
	}
	_, err = usecase.SetBids(client, runCfg, dryRun)
	if err != nil {
		return err
	}
	msg := "watch tick: reconciled"
	if len(priceAdjParts) > 0 {
		msg += " (" + strings.Join(priceAdjParts, "; ") + ")"
	}
	slog.Info(msg, "dry_run", dryRun)
	return nil
}
