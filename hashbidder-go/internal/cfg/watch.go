package cfg

import (
	"time"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"
)

// WatchModeConfig is returned by LoadConfig when [watch].enabled is true in
// explicit-bids mode. Use the watch subcommand to run the timer loop; set-bids
// rejects this shape so one-shot and long-running paths stay distinct.
type WatchModeConfig struct {
	SetBids domain.SetBidsConfig
	Loop    WatchLoopConfig
	Rules   []BidWatchRule
}

// WatchLoopConfig drives the select/sleep loop between reconciliation ticks.
type WatchLoopConfig struct {
	Interval     time.Duration
	Jitter       time.Duration
	InitialDelay time.Duration
}

// StrategyKind selects how a [[bids]] row is repriced on each tick.
type StrategyKind string

const (
	StrategyNone            StrategyKind = ""
	StrategyServedFloorBand StrategyKind = "served_floor_band"
)

// BidWatchRule holds optional automation for one config row (same index as SetBids.Bids).
// StrategyNone means that row keeps the static price from the TOML file.
type BidWatchRule struct {
	Strategy StrategyKind
	MinPrice domain.HashratePrice
	MaxPrice domain.HashratePrice
	// MaxTicksPerStep limits how many tick steps we move toward the goal per tick
	// (reduces chase volatility and API churn).
	MaxTicksPerStep int
}
