package usecase

import (
	"fmt"
	"time"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/bidrunner"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/braiins"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/cfg"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/formatter"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/ocean"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/targethr"
)

func ocean24h(src ocean.Source, addr domain.BtcAddress) (domain.Hashrate, error) {
	stats, err := src.GetAccountStats(addr)
	if err != nil {
		return domain.Hashrate{}, err
	}
	for _, w := range stats.Windows {
		if w.Window == ocean.WindowDay {
			return w.Hashrate, nil
		}
	}
	return domain.Hashrate{}, fmt.Errorf("ocean stats response did not include a 24h window")
}

func SetBidsTarget(
	client braiins.HashpowerClient,
	oceanSrc ocean.Source,
	addr domain.BtcAddress,
	conf cfg.TargetHashrateConfig,
	dryRun bool,
	now time.Time,
) (formatter.SetBidsTargetResult, error) {
	ocean24, err := ocean24h(oceanSrc, addr)
	if err != nil {
		return formatter.SetBidsTargetResult{}, err
	}
	settings, err := client.GetMarketSettings()
	if err != nil {
		return formatter.SetBidsTargetResult{}, err
	}
	book, err := client.GetOrderbook()
	if err != nil {
		return formatter.SetBidsTargetResult{}, err
	}
	price, err := targethr.FindMarketPrice(book, settings.PriceTick)
	if err != nil {
		return formatter.SetBidsTargetResult{}, err
	}
	needed := targethr.ComputeNeededHashrate(conf.TargetHashrate, ocean24)

	current, err := client.GetCurrentBids()
	if err != nil {
		return formatter.SetBidsTargetResult{}, err
	}
	annotated := targethr.CheckCooldowns(current, settings, now)
	bids := targethr.PlanWithCooldowns(price, needed, conf.MaxBidsCount, annotated)
	for _, entry := range bids {
		if err := settings.PriceTick.AssertAligned(entry.Price); err != nil {
			return formatter.SetBidsTargetResult{}, err
		}
	}
	computed := domain.SetBidsConfig{
		DefaultAmount: conf.DefaultAmount,
		Upstream:      conf.Upstream,
		Bids:          bids,
	}
	res, err := bidrunner.Reconcile(client, computed, dryRun, time.Sleep)
	if err != nil {
		return formatter.SetBidsTargetResult{}, err
	}
	return formatter.SetBidsTargetResult{
		Inputs: formatter.TargetHashrateInputsView{
			Ocean24h:      ocean24,
			Target:        conf.TargetHashrate,
			Needed:        needed,
			Price:         price,
			MaxBidsCount:  conf.MaxBidsCount,
			AnnotatedBids: annotated,
		},
		SetBidsResult: res,
	}, nil
}
