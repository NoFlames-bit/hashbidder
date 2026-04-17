package domain

import (
	"fmt"

	"github.com/shopspring/decimal"
)

type PriceTick struct {
	Sats Sats
}

func NewPriceTick(sats Sats) (PriceTick, error) {
	if sats <= 0 {
		return PriceTick{}, fmt.Errorf("price tick must be positive, got %d", sats)
	}
	return PriceTick{Sats: sats}, nil
}

func (t PriceTick) IsAligned(price HashratePrice) bool {
	wire := price.To(EH, Day)
	return int64(wire.Sats)%int64(t.Sats) == 0
}

func (t PriceTick) AssertAligned(price HashratePrice) error {
	if !t.IsAligned(price) {
		return fmt.Errorf("price %s is not aligned to tick %d sat/EH/Day", price.String(), t.Sats)
	}
	return nil
}

func (t PriceTick) AlignDown(price HashratePrice) HashratePrice {
	wire := int64(price.To(EH, Day).Sats)
	ts := int64(t.Sats)
	aligned := (wire / ts) * ts
	per, _ := NewHashrate(decimal.NewFromInt(1), EH, Day)
	return HashratePrice{Sats: Sats(aligned), Per: per}
}

func (t PriceTick) AddOne(price HashratePrice) (HashratePrice, error) {
	if err := t.AssertAligned(price); err != nil {
		return HashratePrice{}, err
	}
	wire := int64(price.To(EH, Day).Sats)
	per, _ := NewHashrate(decimal.NewFromInt(1), EH, Day)
	return HashratePrice{Sats: Sats(wire + int64(t.Sats)), Per: per}, nil
}
