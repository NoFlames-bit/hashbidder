package domain

import (
	"fmt"

	"github.com/shopspring/decimal"
)

const hashratePrecision = 28

func init() {
	decimal.DivisionPrecision = hashratePrecision
}

// HashrateTolerance mirrors Python HASHRATE_TOLERANCE (1E-24 with precision 28).
var HashrateTolerance = decimal.RequireFromString("1e-24")

type Hashrate struct {
	Value    decimal.Decimal
	HashUnit HashUnit
	TimeUnit TimeUnit
}

func NewHashrate(value decimal.Decimal, hu HashUnit, tu TimeUnit) (Hashrate, error) {
	if value.IsNegative() {
		return Hashrate{}, fmt.Errorf("hashrate must be non-negative, got %s", value)
	}
	return Hashrate{Value: value, HashUnit: hu, TimeUnit: tu}, nil
}

func (h Hashrate) asHashesPerSecond() decimal.Decimal {
	num := h.Value.Mul(decimal.NewFromInt(int64(h.HashUnit)))
	den := decimal.NewFromInt(int64(h.TimeUnit))
	return num.Div(den)
}

func (h Hashrate) To(hashUnit HashUnit, timeUnit TimeUnit) Hashrate {
	hps := h.asHashesPerSecond()
	v := hps.Mul(decimal.NewFromInt(int64(timeUnit))).Div(decimal.NewFromInt(int64(hashUnit)))
	return Hashrate{Value: v, HashUnit: hashUnit, TimeUnit: timeUnit}
}

func (h Hashrate) DisplayUnit() Hashrate {
	units := sortedUnitsAsc()
	best := h.To(units[0], h.TimeUnit)
	for _, unit := range units {
		conv := h.To(unit, h.TimeUnit)
		intPart := conv.Value.IntPart()
		if intPart >= 1 && intPart < 1000 {
			best = conv
		}
	}
	if h.asHashesPerSecond().IsZero() {
		z, _ := NewHashrate(decimal.Zero, H, h.TimeUnit)
		return z
	}
	return best
}

func (h Hashrate) String() string {
	tu := "Second"
	switch h.TimeUnit {
	case Minute:
		tu = "Minute"
	case Hour:
		tu = "Hour"
	case Day:
		tu = "Day"
	case Month:
		tu = "Month"
	}
	return fmt.Sprintf("%s %s/%s", h.Value.String(), h.HashUnit.Name(), tu)
}

func (h Hashrate) Add(o Hashrate) Hashrate {
	return Hashrate{
		Value:    h.Value.Add(o.To(h.HashUnit, h.TimeUnit).Value),
		HashUnit: h.HashUnit,
		TimeUnit: h.TimeUnit,
	}
}

func (h Hashrate) Sub(o Hashrate) Hashrate {
	return Hashrate{
		Value:    h.Value.Sub(o.To(h.HashUnit, h.TimeUnit).Value),
		HashUnit: h.HashUnit,
		TimeUnit: h.TimeUnit,
	}
}

func (h Hashrate) Cmp(o Hashrate) int {
	return h.asHashesPerSecond().Cmp(o.asHashesPerSecond())
}

func (h Hashrate) Less(o Hashrate) bool      { return h.Cmp(o) < 0 }
func (h Hashrate) LessEq(o Hashrate) bool    { return h.Cmp(o) <= 0 }
func (h Hashrate) Greater(o Hashrate) bool   { return h.Cmp(o) > 0 }
func (h Hashrate) GreaterEq(o Hashrate) bool { return h.Cmp(o) >= 0 }

// DistributeBids mirrors Python distribute_bids.
func DistributeBids(needed Hashrate, maxBidsCount int) []Hashrate {
	if maxBidsCount < 1 {
		panic(fmt.Sprintf("max_bids_count must be >= 1, got %d", maxBidsCount))
	}
	neededPH := needed.To(PH, Second).Value
	half := decimal.RequireFromString("0.5")
	one := decimal.NewFromInt(1)
	if neededPH.LessThan(half) {
		return nil
	}
	if neededPH.LessThan(one) {
		h, _ := NewHashrate(one, PH, Second)
		return []Hashrate{h}
	}
	n := maxBidsCount
	neededInt := int(neededPH.IntPart())
	if neededInt < n {
		n = neededInt
	}
	if n < 1 {
		return nil
	}
	share := neededPH.Div(decimal.NewFromInt(int64(n))).Round(2)
	out := make([]Hashrate, n)
	for i := 0; i < n; i++ {
		h, _ := NewHashrate(share, PH, Second)
		out[i] = h
	}
	return out
}
