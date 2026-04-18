package watchrun

import (
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/braiins"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/cfg"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/targethr"

	"github.com/shopspring/decimal"
)

func ehPer() domain.Hashrate {
	h, _ := domain.NewHashrate(decimal.NewFromInt(1), domain.EH, domain.Day)
	return h
}

func ehDayFromWire(sats int64) (domain.HashratePrice, error) {
	return domain.NewHashratePrice(domain.Sats(sats), ehPer())
}

func wireSats(p domain.HashratePrice) int64 {
	return int64(p.To(domain.EH, domain.Day).Sats)
}

func clampInt64(x, lo, hi int64) int64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

// stepTowardEH moves live toward target on the tick grid by at most maxSteps
// one-tick hops in EH/Day wire space. Decreases are skipped entirely when allowDown is false.
func stepTowardEH(live, target domain.HashratePrice, tick domain.PriceTick, maxSteps int, allowDown bool) (domain.HashratePrice, error) {
	l := live
	for i := 0; i < maxSteps; i++ {
		lw := wireSats(l)
		tw := wireSats(target)
		if lw == tw {
			return l, nil
		}
		ts := int64(tick.Sats)
		if tw > lw {
			nw := lw + ts
			if nw > tw {
				nw = tw
			}
			np, err := ehDayFromWire(nw)
			if err != nil {
				return domain.HashratePrice{}, err
			}
			l = np
			continue
		}
		if !allowDown {
			return l, nil
		}
		nw := lw - ts
		if nw < tw {
			nw = tw
		}
		np, err := ehDayFromWire(nw)
		if err != nil {
			return domain.HashratePrice{}, err
		}
		l = np
	}
	return l, nil
}

// ApplyServedFloorBand moves the live bid price toward a band around the
// competitive undercut of the served stack: too low vs served bids risks
// starvation; too high burns margin. Decreases respect priceCooldown (Braiins
// decrease windows); increases are always attempted in steps bounded by
// max_ticks_per_step.
func ApplyServedFloorBand(
	rule cfg.BidWatchRule,
	tick domain.PriceTick,
	book braiins.OrderBook,
	live domain.HashratePrice,
	priceCooldown bool,
) (domain.HashratePrice, bool, error) {
	if rule.Strategy != cfg.StrategyServedFloorBand {
		return live, false, nil
	}
	servedUndercut, err := targethr.FindMarketPrice(book, tick)
	if err != nil {
		return live, false, nil
	}
	live = tick.AlignDown(live)
	minW := wireSats(rule.MinPrice)
	maxW := wireSats(rule.MaxPrice)
	servW := wireSats(servedUndercut)
	anchorW := clampInt64(servW, minW, maxW)
	anchor, err := ehDayFromWire(anchorW)
	if err != nil {
		return domain.HashratePrice{}, false, err
	}
	allowDown := !priceCooldown
	next, err := stepTowardEH(live, anchor, tick, rule.MaxTicksPerStep, allowDown)
	if err != nil {
		return domain.HashratePrice{}, false, err
	}
	if wireSats(next) == wireSats(live) {
		return live, false, nil
	}
	// Hard clamp to configured band (defensive).
	nw := clampInt64(wireSats(next), minW, maxW)
	out, err := ehDayFromWire(nw)
	if err != nil {
		return domain.HashratePrice{}, false, err
	}
	if wireSats(out) == wireSats(live) {
		return live, false, nil
	}
	return out, true, nil
}
