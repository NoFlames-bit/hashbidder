package watchrun

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/braiins"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/cfg"
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

func nextWait(loop cfg.WatchLoopConfig) time.Duration {
	base := loop.Interval
	if loop.Jitter <= 0 {
		return base
	}
	return base + time.Duration(rand.Int64N(int64(loop.Jitter)+1))
}

// Run executes OneTick on a timer until ctx is cancelled. Cooldown-aware
// strategies rely on ResolveCooldowns inside each tick (same as target mode).
func Run(ctx context.Context, client braiins.HashpowerClient, wm *cfg.WatchModeConfig, dryRun bool) error {
	if wm.Loop.InitialDelay > 0 {
		slog.Info("watch: initial delay", "duration", wm.Loop.InitialDelay)
		if err := sleepOrDone(ctx, wm.Loop.InitialDelay); err != nil {
			return err
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
		if err := sleepOrDone(ctx, wait); err != nil {
			return err
		}
	}
}
