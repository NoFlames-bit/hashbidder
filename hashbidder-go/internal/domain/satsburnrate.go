package domain

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

type SatsBurnRate struct {
	Amount decimal.Decimal
	Period time.Duration
}

func ZeroBurnRate() SatsBurnRate {
	return SatsBurnRate{Amount: decimal.Zero, Period: 24 * time.Hour}
}

func NewSatsBurnRate(amount decimal.Decimal, period time.Duration) (SatsBurnRate, error) {
	if amount.IsNegative() {
		return SatsBurnRate{}, fmt.Errorf("sats burn rate amount must be non-negative")
	}
	if period <= 0 {
		return SatsBurnRate{}, fmt.Errorf("sats burn rate period must be positive")
	}
	return SatsBurnRate{Amount: amount, Period: period}, nil
}

func (s SatsBurnRate) To(period time.Duration) SatsBurnRate {
	scale := decimal.NewFromInt(period.Nanoseconds()).Div(decimal.NewFromInt(s.Period.Nanoseconds()))
	return SatsBurnRate{Amount: s.Amount.Mul(scale), Period: period}
}

func (s SatsBurnRate) Runway(available Sats) time.Duration {
	if s.Amount.IsZero() {
		return time.Duration(1<<63 - 1) // sentinel "infinite" for formatting
	}
	if available <= 0 {
		return 0
	}
	// runway = available * period / amount (sats per period)
	ns := decimal.NewFromInt(int64(available)).Mul(decimal.NewFromInt(s.Period.Nanoseconds())).Div(s.Amount).Round(0)
	bi := ns.BigInt()
	if !bi.IsInt64() {
		return time.Duration(1<<63 - 1)
	}
	n := bi.Int64()
	if n < 0 {
		return 0
	}
	return time.Duration(n)
}

func (s SatsBurnRate) Add(o SatsBurnRate) SatsBurnRate {
	return SatsBurnRate{
		Amount: s.Amount.Add(o.To(s.Period).Amount),
		Period: s.Period,
	}
}
