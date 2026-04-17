package usecase

import (
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/bidrunner"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/braiins"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"
	"time"
)

func SetBids(client braiins.HashpowerClient, cfg domain.SetBidsConfig, dryRun bool) (*bidrunner.SetBidsResult, error) {
	return bidrunner.Reconcile(client, cfg, dryRun, time.Sleep)
}
