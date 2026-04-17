package usecase

import (
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/hv"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/mempool"
)

func GetHashvalue(src mempool.Source) (hv.Components, error) {
	stats, err := src.GetChainStats(domain.BlocksPerEpoch)
	if err != nil {
		return hv.Components{}, err
	}
	return hv.ComputeHashvalue(stats.Difficulty, stats.TipHeight, stats.TotalFee)
}
