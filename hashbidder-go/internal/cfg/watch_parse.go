package cfg

import (
	"fmt"
	"strings"
	"time"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"

	"github.com/shopspring/decimal"
)

type rawWatch struct {
	Enabled             bool  `toml:"enabled"`
	IntervalSeconds     int64 `toml:"interval_seconds"`
	JitterSeconds       int64 `toml:"jitter_seconds"`
	InitialDelaySeconds int64 `toml:"initial_delay_seconds"`
}

func parseWatchMode(data *rawFile, setBids domain.SetBidsConfig, bids []domain.BidConfig) (*WatchModeConfig, error) {
	if data.Watch == nil || !data.Watch.Enabled {
		return nil, nil
	}
	if len(bids) == 0 {
		return nil, fmt.Errorf("[watch]: requires at least one [[bids]] row")
	}
	if len(data.Bids) != len(bids) {
		return nil, fmt.Errorf("internal: raw bids length mismatch")
	}
	interval := time.Duration(data.Watch.IntervalSeconds) * time.Second
	if data.Watch.IntervalSeconds < 15 {
		return nil, fmt.Errorf("[watch].interval_seconds must be >= 15 (got %d)", data.Watch.IntervalSeconds)
	}
	jitter := time.Duration(data.Watch.JitterSeconds) * time.Second
	if data.Watch.JitterSeconds < 0 {
		return nil, fmt.Errorf("[watch].jitter_seconds must be >= 0")
	}
	initDelay := time.Duration(data.Watch.InitialDelaySeconds) * time.Second
	if data.Watch.InitialDelaySeconds < 0 {
		return nil, fmt.Errorf("[watch].initial_delay_seconds must be >= 0")
	}

	rules := make([]BidWatchRule, len(data.Bids))
	anyStrategy := false
	for i, rb := range data.Bids {
		sk, err := parseStrategyKind(rb.WatchStrategy)
		if err != nil {
			return nil, fmt.Errorf("bid %d: %w", i, err)
		}
		if sk == StrategyNone {
			rules[i] = BidWatchRule{Strategy: StrategyNone, MaxTicksPerStep: 1}
			continue
		}
		anyStrategy = true
		if rb.PriceMinSatPerPHDay == nil || rb.PriceMaxSatPerPHDay == nil {
			return nil, fmt.Errorf("bid %d: watch_strategy %q requires price_min_sat_per_ph_day and price_max_sat_per_ph_day", i, sk)
		}
		minSat, err := parseIntField(fmt.Sprintf("bid %d price_min_sat_per_ph_day", i), rb.PriceMinSatPerPHDay)
		if err != nil {
			return nil, err
		}
		maxSat, err := parseIntField(fmt.Sprintf("bid %d price_max_sat_per_ph_day", i), rb.PriceMaxSatPerPHDay)
		if err != nil {
			return nil, err
		}
		if minSat <= 0 || maxSat <= 0 || minSat > maxSat {
			return nil, fmt.Errorf("bid %d: invalid price min/max (min=%d max=%d)", i, minSat, maxSat)
		}
		minP, _ := domain.NewHashratePrice(domain.Sats(minSat), mustHR(decimal.NewFromInt(1), domain.PH, domain.Day))
		maxP, _ := domain.NewHashratePrice(domain.Sats(maxSat), mustHR(decimal.NewFromInt(1), domain.PH, domain.Day))
		maxTicks := 1
		if rb.MaxTicksPerStep != nil {
			mt, err := parseIntField(fmt.Sprintf("bid %d max_ticks_per_step", i), rb.MaxTicksPerStep)
			if err != nil {
				return nil, err
			}
			if mt < 1 || mt > 50 {
				return nil, fmt.Errorf("bid %d: max_ticks_per_step must be in [1,50]", i)
			}
			maxTicks = int(mt)
		}
		rules[i] = BidWatchRule{
			Strategy:        sk,
			MinPrice:        minP,
			MaxPrice:        maxP,
			MaxTicksPerStep: maxTicks,
		}
	}
	if !anyStrategy {
		return nil, fmt.Errorf("[watch].enabled requires at least one [[bids]] row with watch_strategy set")
	}

	return &WatchModeConfig{
		SetBids: setBids,
		Loop: WatchLoopConfig{
			Interval:     interval,
			Jitter:       jitter,
			InitialDelay: initDelay,
		},
		Rules: rules,
	}, nil
}

func parseStrategyKind(s *string) (StrategyKind, error) {
	if s == nil || strings.TrimSpace(*s) == "" {
		return StrategyNone, nil
	}
	switch strings.ToLower(strings.TrimSpace(*s)) {
	case string(StrategyServedFloorBand):
		return StrategyServedFloorBand, nil
	default:
		return "", fmt.Errorf("unknown watch_strategy %q (supported: %q)", *s, StrategyServedFloorBand)
	}
}
