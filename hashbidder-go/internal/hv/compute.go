package hv

import (
	"errors"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"

	"github.com/shopspring/decimal"
)

var ErrZeroDifficulty = errors.New("difficulty is zero")

type Components struct {
	TipHeight       domain.BlockHeight
	Subsidy         domain.Sats
	TotalFees       domain.Sats
	TotalReward     domain.Sats
	Difficulty      decimal.Decimal
	NetworkHashrate decimal.Decimal
	Hashvalue       domain.HashratePrice
}

func ComputeHashvalue(difficulty decimal.Decimal, tip domain.BlockHeight, totalFees domain.Sats) (Components, error) {
	if difficulty.IsZero() {
		return Components{}, ErrZeroDifficulty
	}
	subsidy := domain.BlockSubsidy(tip)

	tr := decimal.NewFromInt(int64(subsidy)).
		Mul(decimal.NewFromInt(int64(domain.BlocksPerEpoch))).
		Add(decimal.NewFromInt(int64(totalFees)))
	avgReward := tr.Div(decimal.NewFromInt(int64(domain.BlocksPerEpoch))).RoundBank(0)

	two32 := decimal.NewFromInt(4294967296) // 2^32
	networkHR := difficulty.Mul(two32).Div(decimal.NewFromInt(domain.BlockTimeSeconds))

	phMul := decimal.NewFromInt(int64(domain.PH))
	blocksPerDay := decimal.NewFromInt(domain.BlocksPerDay)
	hashvalue := avgReward.Mul(blocksPerDay).Mul(phMul).Div(networkHR)

	hvPrice, err := domain.NewHashratePrice(domain.Sats(hashvalue.RoundBank(0).IntPart()), mustHR(decimal.NewFromInt(1), domain.PH, domain.Day))
	if err != nil {
		return Components{}, err
	}
	return Components{
		TipHeight:       tip,
		Subsidy:         subsidy,
		TotalFees:       totalFees,
		TotalReward:     domain.Sats(tr.IntPart()),
		Difficulty:      difficulty,
		NetworkHashrate: networkHR,
		Hashvalue:       hvPrice,
	}, nil
}

func mustHR(v decimal.Decimal, hu domain.HashUnit, tu domain.TimeUnit) domain.Hashrate {
	h, err := domain.NewHashrate(v, hu, tu)
	if err != nil {
		panic(err)
	}
	return h
}
