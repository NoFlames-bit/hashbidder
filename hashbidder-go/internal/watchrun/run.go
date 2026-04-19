package watchrun

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/braiins"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/cfg"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/usecase"
)

func sleepOrDone(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func stopTimerDrain(t *time.Timer) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
}

func loadWatchMode(path string) (*cfg.WatchModeConfig, error) {
	confAny, err := cfg.LoadConfig(path)
	if err != nil {
		return nil, err
	}
	wm, ok := confAny.(cfg.WatchModeConfig)
	if !ok {
		return nil, fmt.Errorf("need explicit bids with [watch].enabled = true (got %T)", confAny)
	}
	return &wm, nil
}

// applyConfigReload re-reads path, reconciles to the new desired state, then swaps *wm
// on success. On load or reconcile failure, previous *wm is unchanged and a single log line
// records the specific failure.
func applyConfigReload(ctx context.Context, client braiins.HashpowerClient, path string, wm *cfg.WatchModeConfig, dryRun bool) {
	if err := ctx.Err(); err != nil {
		return
	}
	next, err := loadWatchMode(path)
	if err != nil {
		slog.Error("watch: SIGHUP reload kept previous settings unchanged",
			"path", path,
			"failure", fmt.Sprintf("could not load or validate config file: %v", err))
		return
	}
	res, err := usecase.SetBids(client, next.SetBids, dryRun)
	if err != nil {
		slog.Error("watch: SIGHUP reload kept previous settings unchanged",
			"path", path,
			"failure", fmt.Sprintf("reconcile failed: %v", err))
		return
	}
	if !dryRun && res.BalanceCheck.Status == domain.BalanceInsufficient {
		slog.Error("watch: SIGHUP reload kept previous settings unchanged",
			"path", path,
			"failure", fmt.Sprintf("reconcile blocked: insufficient balance (required %d sat, available %d sat, status=%s)",
				int64(res.BalanceCheck.RequiredSat), int64(res.BalanceCheck.AvailableSat), res.BalanceCheck.Status))
		return
	}
	*wm = *next
	slog.Info("watch: config reloaded (SIGHUP)", "path", path,
		"interval", wm.Loop.Interval, "jitter", wm.Loop.Jitter,
		"bids", len(wm.SetBids.Bids), "dry_run", dryRun)
}

func nextWait(loop cfg.WatchLoopConfig) time.Duration {
	base := loop.Interval
	if loop.Jitter <= 0 {
		return base
	}
	return base + time.Duration(rand.Int64N(int64(loop.Jitter)+1))
}

// Run executes OneTick on a timer until ctx is cancelled. Cooldown-aware
// strategies rely on ResolveCooldowns inside each tick (same as target mode).
//
// Sending SIGHUP reloads configPath (same shape as startup), runs a full reconcile
// for the new desired bids, then continues the loop with the updated config.
func Run(ctx context.Context, client braiins.HashpowerClient, configPath string, wm *cfg.WatchModeConfig, dryRun bool) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP)
	defer signal.Stop(sigCh)

	reloadCh := make(chan struct{}, 1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-sigCh:
				select {
				case reloadCh <- struct{}{}:
				default:
				}
			}
		}
	}()

	if wm.Loop.InitialDelay > 0 {
		slog.Info("watch: initial delay", "duration", wm.Loop.InitialDelay)
		t := time.NewTimer(wm.Loop.InitialDelay)
		select {
		case <-ctx.Done():
			stopTimerDrain(t)
			return ctx.Err()
		case <-reloadCh:
			stopTimerDrain(t)
			applyConfigReload(ctx, client, configPath, wm, dryRun)
		case <-t.C:
		}
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := OneTick(ctx, client, wm, dryRun); err != nil {
			slog.Error("watch tick failed", "err", err)
		}
		wait := nextWait(wm.Loop)
		nextRun := time.Now().Add(wait).In(time.Local).Format("3:04:05pm")
		slog.Debug("watch: sleeping, next run "+nextRun, "interval", wait)
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			stopTimerDrain(t)
			return ctx.Err()
		case <-reloadCh:
			stopTimerDrain(t)
			applyConfigReload(ctx, client, configPath, wm, dryRun)
		case <-t.C:
		}
	}
}
