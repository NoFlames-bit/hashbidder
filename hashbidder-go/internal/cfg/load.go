package cfg

import (
	"fmt"
	"os"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"

	"github.com/pelletier/go-toml/v2"
	"github.com/shopspring/decimal"
)

type ConfigMode string

const (
	ExplicitBids   ConfigMode = "explicit-bids"
	TargetHashrate ConfigMode = "target-hashrate"
)

type TargetHashrateConfig struct {
	DefaultAmount  domain.Sats
	Upstream       domain.Upstream
	TargetHashrate domain.Hashrate
	MaxBidsCount   int
}

type rawUpstream struct {
	URL      string `toml:"url"`
	Identity string `toml:"identity"`
}

type rawBid struct {
	PriceSatPerPHDay any `toml:"price_sat_per_ph_day"`
	SpeedLimitPHS    any `toml:"speed_limit_ph_s"`
}

type rawFile struct {
	Mode              *string     `toml:"mode"`
	DefaultAmountSat  any         `toml:"default_amount_sat"`
	Upstream          rawUpstream `toml:"upstream"`
	Bids              []rawBid    `toml:"bids"`
	TargetHashratePHS any         `toml:"target_hashrate_ph_s"`
	MaxBidsCount      any         `toml:"max_bids_count"`
}

func parseIntField(label string, v any) (int64, error) {
	switch x := v.(type) {
	case int64:
		return x, nil
	case int:
		return int64(x), nil
	case uint64:
		return int64(x), nil
	default:
		if label == "" {
			return 0, fmt.Errorf("must be an integer")
		}
		return 0, fmt.Errorf("%s must be an integer", label)
	}
}

func anyToDecimal(v any) (decimal.Decimal, error) {
	switch x := v.(type) {
	case int64:
		return decimal.NewFromInt(x), nil
	case int:
		return decimal.NewFromInt(int64(x)), nil
	case uint64:
		return decimal.NewFromUint64(x), nil
	case float64:
		return decimal.NewFromFloat(x), nil
	case string:
		return decimal.NewFromString(x)
	default:
		return decimal.Zero, fmt.Errorf("expected number, got %T", v)
	}
}

func LoadConfig(path string) (any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var data rawFile
	if err := toml.Unmarshal(b, &data); err != nil {
		return nil, fmt.Errorf("invalid TOML: %w", err)
	}
	mode := ExplicitBids
	if data.Mode != nil {
		switch ConfigMode(*data.Mode) {
		case ExplicitBids, TargetHashrate:
			mode = ConfigMode(*data.Mode)
		default:
			return nil, fmt.Errorf("invalid mode %q: must be one of %q, %q", *data.Mode, ExplicitBids, TargetHashrate)
		}
	}
	if data.DefaultAmountSat == nil {
		return nil, fmt.Errorf("missing required field: default_amount_sat")
	}
	defAmt, err := parseIntField("default_amount_sat", data.DefaultAmountSat)
	if err != nil {
		return nil, err
	}
	if data.Upstream.URL == "" {
		return nil, fmt.Errorf("missing required upstream field: url")
	}
	if data.Upstream.Identity == "" {
		return nil, fmt.Errorf("missing required upstream field: identity")
	}
	su, err := domain.ParseStratumURL(data.Upstream.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid upstream URL: %w", err)
	}
	up := domain.Upstream{URL: su, Identity: data.Upstream.Identity}
	def := domain.Sats(defAmt)

	if mode == TargetHashrate {
		if len(data.Bids) > 0 {
			return nil, fmt.Errorf("target-hashrate mode does not accept [[bids]] sections")
		}
		if data.TargetHashratePHS == nil {
			return nil, fmt.Errorf("missing required field: target_hashrate_ph_s")
		}
		th, err := anyToDecimal(data.TargetHashratePHS)
		if err != nil {
			return nil, fmt.Errorf("target_hashrate_ph_s must be a number")
		}
		if !th.IsPositive() {
			return nil, fmt.Errorf("target_hashrate_ph_s must be positive")
		}
		if data.MaxBidsCount == nil {
			return nil, fmt.Errorf("missing required field: max_bids_count")
		}
		maxBC, err := parseIntField("max_bids_count", data.MaxBidsCount)
		if err != nil {
			return nil, err
		}
		if maxBC < 1 {
			return nil, fmt.Errorf("max_bids_count must be >= 1")
		}
		thr, err := domain.NewHashrate(th, domain.PH, domain.Second)
		if err != nil {
			return nil, err
		}
		return TargetHashrateConfig{
			DefaultAmount:  def,
			Upstream:       up,
			TargetHashrate: thr,
			MaxBidsCount:   int(maxBC),
		}, nil
	}

	bids := make([]domain.BidConfig, 0, len(data.Bids))
	for i, bd := range data.Bids {
		if bd.PriceSatPerPHDay == nil {
			return nil, fmt.Errorf("bid %d: missing required field: price_sat_per_ph_day", i)
		}
		priceInt, err := parseIntField(fmt.Sprintf("bid %d price_sat_per_ph_day", i), bd.PriceSatPerPHDay)
		if err != nil {
			return nil, err
		}
		if priceInt == 0 {
			return nil, fmt.Errorf("bid %d: missing required field: price_sat_per_ph_day", i)
		}
		if bd.SpeedLimitPHS == nil {
			return nil, fmt.Errorf("bid %d: missing required field: speed_limit_ph_s", i)
		}
		spd, err := anyToDecimal(bd.SpeedLimitPHS)
		if err != nil {
			return nil, fmt.Errorf("bid %d: speed_limit_ph_s must be a number", i)
		}
		if !spd.IsPositive() {
			return nil, fmt.Errorf("bid %d: speed_limit_ph_s must be positive", i)
		}
		price, _ := domain.NewHashratePrice(domain.Sats(priceInt), mustHR(decimal.NewFromInt(1), domain.PH, domain.Day))
		slim, err := domain.NewHashrate(spd, domain.PH, domain.Second)
		if err != nil {
			return nil, err
		}
		bids = append(bids, domain.BidConfig{Price: price, SpeedLimit: slim})
	}
	return domain.SetBidsConfig{
		DefaultAmount: def,
		Upstream:      up,
		Bids:          bids,
	}, nil
}

func mustHR(v decimal.Decimal, hu domain.HashUnit, tu domain.TimeUnit) domain.Hashrate {
	h, err := domain.NewHashrate(v, hu, tu)
	if err != nil {
		panic(err)
	}
	return h
}
