package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type BalanceStatus string

const (
	BalanceSufficient   BalanceStatus = "sufficient"
	BalanceLow          BalanceStatus = "low"
	BalanceInsufficient BalanceStatus = "insufficient"
)

var LowBalanceRunway = 72 * time.Hour

type BalanceCheck struct {
	RequiredSat    Sats
	AvailableSat   Sats
	BurnRate       SatsBurnRate
	Runway         time.Duration
	Status         BalanceStatus
	RunwayInfinite bool
}

func planBurnRate(plan ReconciliationPlan) SatsBurnRate {
	total := ZeroBurnRate()
	oneDay := 24 * time.Hour
	for _, c := range plan.Creates {
		// Keep time units consistent: (EH/s) * (sat/EH/day) = sat/day.
		// Using EH/day here would multiply by 86_400 twice and inflate burn rate.
		speedEHPerSecond := c.Config.SpeedLimit.To(EH, Second).Value
		priceSatPerEHDay := decimal.NewFromInt(int64(c.Config.Price.To(EH, Day).Sats))
		br, _ := NewSatsBurnRate(speedEHPerSecond.Mul(priceSatPerEHDay), oneDay)
		total = total.Add(br)
	}
	return total
}

func CheckBalance(plan ReconciliationPlan, availableSat Sats) BalanceCheck {
	var required int64
	for _, c := range plan.Creates {
		required += int64(c.Amount)
	}
	burn := planBurnRate(plan)
	runway := burn.Runway(availableSat)
	infinite := burn.Amount.IsZero()
	var status BalanceStatus
	switch {
	case availableSat < Sats(required):
		status = BalanceInsufficient
	case !infinite && runway < LowBalanceRunway:
		status = BalanceLow
	default:
		status = BalanceSufficient
	}
	return BalanceCheck{
		RequiredSat:    Sats(required),
		AvailableSat:   availableSat,
		BurnRate:       burn,
		Runway:         runway,
		Status:         status,
		RunwayInfinite: infinite,
	}
}
