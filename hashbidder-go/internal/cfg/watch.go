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
	// StrategyServedDepthBand undercuts the served book like served_floor_band,
	// but the competitive tier is chosen only after cumulative hr_matched from
	// the cheapest served levels reaches ServedLiquidityFloor (see TOML).
	StrategyServedDepthBand StrategyKind = "served_depth_band"
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
	// ServedLiquidityFloor is used by StrategyServedDepthBand: cumulative
	// hr_matched_ph from the bottom of the served stack must reach this before
	// we treat a price tier as the anchor (same units as order book hr_matched_ph).
	ServedLiquidityFloor domain.Hashrate
}
