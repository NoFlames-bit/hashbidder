package usecase

import (
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/ocean"
)

func GetOceanAccountStats(src ocean.Source, addr domain.BtcAddress) (ocean.AccountStats, error) {
	return src.GetAccountStats(addr)
}
