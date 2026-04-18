package targethr

import (
	"errors"
	"fmt"
	"time"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/braiins"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"

	"github.com/shopspring/decimal"
)

func ComputeNeededHashrate(target, current24h domain.Hashrate) domain.Hashrate {
	twice := target.Add(target)
	if current24h.GreaterEq(twice) {
		h, _ := domain.NewHashrate(decimal.Zero, domain.PH, domain.Second)
		return h
	}
	return twice.Sub(current24h).To(domain.PH, domain.Second)
}

type CooldownInfo struct {
	PriceCooldown bool
	SpeedCooldown bool
}

type BidWithCooldown struct {
	Bid      domain.UserBid
	Cooldown CooldownInfo
}

// IsPriceGuaranteedFree reports whether the bid's price is provably past its
// decrease window using only LastUpdated (any user update bumps it). False
// means "unknown" and may require bid history.
func IsPriceGuaranteedFree(bid domain.UserBid, settings braiins.MarketSettings, now time.Time) bool {
	return now.Sub(bid.LastUpdated) >= settings.MinBidPriceDecreasePeriod
}

// IsSpeedGuaranteedFree reports whether the bid's speed limit is provably
// past its decrease window using only LastUpdated.
func IsSpeedGuaranteedFree(bid domain.UserBid, settings braiins.MarketSettings, now time.Time) bool {
	return now.Sub(bid.LastUpdated) >= settings.MinBidSpeedLimitDecreasePeriod
}

// CooldownFromHistory derives per-field cooldown flags from bid history and
// market decrease windows. A flag is true iff the last strict decrease of that
// field occurred within its cooldown period.
func CooldownFromHistory(history domain.BidHistory, settings braiins.MarketSettings, now time.Time) CooldownInfo {
	lastPrice := history.LastPriceDecreaseAt()
	lastSpeed := history.LastSpeedDecreaseAt()
	priceCD := lastPrice != nil && now.Sub(*lastPrice) < settings.MinBidPriceDecreasePeriod
	speedCD := lastSpeed != nil && now.Sub(*lastSpeed) < settings.MinBidSpeedLimitDecreasePeriod
	return CooldownInfo{PriceCooldown: priceCD, SpeedCooldown: speedCD}
}

// ResolveCooldowns annotates each bid with cooldown state: tier-1 predicates
// skip history when both fields are provably free; otherwise fetches bid
// history. On *braiins.APIError from history, uses a conservative fallback
// (each field in cooldown unless its tier-1 predicate proved it free). Other
// errors are returned.
func ResolveCooldowns(client braiins.HashpowerClient, bids []domain.UserBid, settings braiins.MarketSettings, now time.Time) ([]BidWithCooldown, error) {
	out := make([]BidWithCooldown, 0, len(bids))
	for _, bid := range bids {
		priceFree := IsPriceGuaranteedFree(bid, settings, now)
		speedFree := IsSpeedGuaranteedFree(bid, settings, now)
		var cd CooldownInfo
		if priceFree && speedFree {
			cd = CooldownInfo{}
		} else {
			history, err := client.GetBidHistory(bid.ID)
			if err != nil {
				var apiErr *braiins.APIError
				if errors.As(err, &apiErr) {
					cd = CooldownInfo{
						PriceCooldown: !priceFree,
						SpeedCooldown: !speedFree,
					}
				} else {
					return nil, fmt.Errorf("get bid history %s: %w", bid.ID, err)
				}
			} else {
				cd = CooldownFromHistory(history, settings, now)
			}
		}
		out = append(out, BidWithCooldown{Bid: bid, Cooldown: cd})
	}
	return out, nil
}

func PlanWithCooldowns(desiredPrice domain.HashratePrice, needed domain.Hashrate, maxBids int, bids []BidWithCooldown) []domain.BidConfig {
	speedLocked := make([]BidWithCooldown, 0)
	priceLockedOnly := make([]BidWithCooldown, 0)
	for _, b := range bids {
		if b.Cooldown.SpeedCooldown {
			speedLocked = append(speedLocked, b)
		} else if b.Cooldown.PriceCooldown {
			priceLockedOnly = append(priceLockedOnly, b)
		}
	}

	lockedSpeedTotal, _ := domain.NewHashrate(decimal.Zero, domain.PH, domain.Second)
	for _, e := range speedLocked {
		lockedSpeedTotal = lockedSpeedTotal.Add(e.Bid.SpeedLimitPH)
	}

	lockedEntries := make([]domain.BidConfig, 0, len(speedLocked))
	for _, e := range speedLocked {
		price := desiredPrice
		if e.Cooldown.PriceCooldown {
			price = e.Bid.Price
		}
		lockedEntries = append(lockedEntries, domain.BidConfig{
			Price:      price,
			SpeedLimit: e.Bid.SpeedLimitPH,
		})
	}

	var remaining domain.Hashrate
	if needed.Greater(lockedSpeedTotal) {
		remaining = needed.Sub(lockedSpeedTotal)
	} else {
		remaining, _ = domain.NewHashrate(decimal.Zero, domain.PH, domain.Second)
	}

	remainingSlots := maxBids - len(speedLocked)
	if remainingSlots < 0 {
		remainingSlots = 0
	}
	speeds := []domain.Hashrate{}
	if remainingSlots > 0 {
		speeds = domain.DistributeBids(remaining, remainingSlots)
	}

	free := make([]domain.BidConfig, 0, len(speeds))
	for i, speed := range speeds {
		if i < len(priceLockedOnly) {
			entry := priceLockedOnly[i]
			free = append(free, domain.BidConfig{Price: entry.Bid.Price, SpeedLimit: speed})
		} else {
			free = append(free, domain.BidConfig{Price: desiredPrice, SpeedLimit: speed})
		}
	}

	out := append(lockedEntries, free...)
	return out
}

func FindMarketPrice(orderbook braiins.OrderBook, tick domain.PriceTick) (domain.HashratePrice, error) {
	var cheapest *braiins.BidItem
	for i := range orderbook.Bids {
		b := &orderbook.Bids[i]
		if b.HrMatchedPH.Value.IsPositive() {
			if cheapest == nil || b.Price.Sats < cheapest.Price.Sats {
				c := *b
				cheapest = &c
			}
		}
	}
	if cheapest == nil {
		return domain.HashratePrice{}, errors.New("order book has no served bids; cannot pick a price")
	}
	aligned := tick.AlignDown(cheapest.Price)
	return tick.AddOne(aligned)
}
