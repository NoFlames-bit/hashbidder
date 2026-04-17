package domain

import (
	"fmt"

	"github.com/shopspring/decimal"
)

type HashratePrice struct {
	Sats Sats
	Per  Hashrate
}

func NewHashratePrice(sats Sats, per Hashrate) (HashratePrice, error) {
	if sats < 0 {
		return HashratePrice{}, fmt.Errorf("hashrate price must be non-negative, got %d sats", sats)
	}
	return HashratePrice{Sats: sats, Per: per}, nil
}

func (p HashratePrice) To(hashUnit HashUnit, timeUnit TimeUnit) HashratePrice {
	newPer, _ := NewHashrate(decimal.NewFromInt(1), hashUnit, timeUnit)
	dstHash := decimal.NewFromInt(int64(hashUnit))
	dstTime := decimal.NewFromInt(int64(timeUnit))
	srcHash := decimal.NewFromInt(int64(p.Per.HashUnit))
	srcTime := decimal.NewFromInt(int64(p.Per.TimeUnit))

	multiplier := dstHash.Mul(srcTime).Div(dstTime.Mul(srcHash).Mul(p.Per.Value))
	scaled := decimal.NewFromInt(int64(p.Sats)).Mul(multiplier)
	satsOut := scaled.RoundBank(0).IntPart()
	return HashratePrice{Sats: Sats(satsOut), Per: newPer}
}

func (p HashratePrice) String() string {
	return fmt.Sprintf("%d sat/%s", p.Sats, p.Per.String())
}
