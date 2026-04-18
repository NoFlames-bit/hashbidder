package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/braiins"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/cfg"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/formatter"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/mempool"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/ocean"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/usecase"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/watchrun"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var (
	verbose bool
	logFile string
	dryRun  bool
)

func setupLogging() {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	opts := &slog.HandlerOptions{Level: level}
	std := slog.NewTextHandler(os.Stderr, opts)
	if logFile == "" {
		slog.SetDefault(slog.New(std))
		return
	}
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot open log file: %v\n", err)
		os.Exit(1)
	}
	file := slog.NewTextHandler(f, opts)
	slog.SetDefault(slog.New(&teeHandler{stderr: std, file: file}))
}

func logInvocation() {
	if logFile == "" {
		return
	}
	// Emit a startup marker when file logging is enabled so each run is traceable.
	slog.Info("hashbidder run", "argv", strings.Join(os.Args[1:], " "))
}

func printResult(msg string) {
	fmt.Println(msg)
	if logFile != "" {
		slog.Info("hashbidder result", "output", msg)
	}
}

type teeHandler struct {
	stderr slog.Handler
	file   slog.Handler
}

func (t *teeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return t.stderr.Enabled(ctx, level) || t.file.Enabled(ctx, level)
}

func (t *teeHandler) Handle(ctx context.Context, r slog.Record) error {
	_ = t.stderr.Handle(ctx, r)
	return t.file.Handle(ctx, r)
}

func (t *teeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &teeHandler{stderr: t.stderr.WithAttrs(attrs), file: t.file.WithAttrs(attrs)}
}

func (t *teeHandler) WithGroup(name string) slog.Handler {
	return &teeHandler{stderr: t.stderr.WithGroup(name), file: t.file.WithGroup(name)}
}

func httpClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

func braiinsKey() *string {
	v := strings.TrimSpace(os.Getenv("BRAIINS_API_KEY"))
	if v == "" {
		return nil
	}
	return &v
}

func mempoolBase() string {
	if v := strings.TrimSpace(os.Getenv("MEMPOOL_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return mempool.DefaultMempoolURL
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "hashbidder",
		Short: "Braiins Hashpower spot bid helper (Go port)",
		Long: `hashbidder talks to Braiins Hashpower (and optionally OCEAN / mempool) to inspect or reconcile spot bids.

Commands:
  ping                 Connectivity check (order book)
  bids                 List current bids
  hashvalue            Expected sat/PH/Day from chain data
  ocean-account-stats  OCEAN pool HTML stats (needs OCEAN_ADDRESS)
  set-bids             One-shot reconcile from TOML (explicit bids or target-hashrate mode)
  watch                Long-running reconcile loop from TOML (explicit bids + [watch] section)

Global flags apply to all commands. Use "hashbidder <command> --help" for command-specific flags.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			_ = godotenv.Load()
			setupLogging()
			logInvocation()
		},
	}

	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable debug logging")
	root.PersistentFlags().StringVar(&logFile, "log-file", "", "Also log to this file")
	root.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "For set-bids and watch: plan each reconcile without cancel/edit/create (reads still run; target/watch may call bid-detail history)")

	ping := &cobra.Command{
		Use:   "ping",
		Short: "Check connectivity to Braiins Hashpower API",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := braiins.NewClient(braiins.APIBase, braiinsKey(), httpClient())
			book, err := c.GetOrderbook()
			if err != nil {
				return err
			}
			slog.Debug("order book", "bids", len(book.Bids), "asks", len(book.Asks))
			printResult(fmt.Sprintf("OK — order book: %d bids, %d asks", len(book.Bids), len(book.Asks)))
			return nil
		},
	}

	bids := &cobra.Command{
		Use:   "bids",
		Short: "List your currently active bids",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := braiins.NewClient(braiins.APIBase, braiinsKey(), httpClient())
			cur, err := c.GetCurrentBids()
			if err != nil {
				return err
			}
			if len(cur) == 0 {
				printResult("No active bids.")
				return nil
			}
			for _, bid := range cur {
				price := bid.Price.To(domain.PH, domain.Day)
				rem := "-"
				if bid.AmountRemainingSat != nil {
					rem = fmt.Sprintf("%d", *bid.AmountRemainingSat)
				}
				prog := "-"
				if bid.Progress != nil {
					prog = bid.Progress.String()
				}
				printResult(fmt.Sprintf("%s  %14s  price=%s  limit=%s  remaining=%s sat  progress=%s",
					bid.ID, bid.Status, price.String(), bid.SpeedLimitPH.String(), rem, prog))
			}
			return nil
		},
	}

	hashvalue := &cobra.Command{
		Use:   "hashvalue",
		Short: "Compute hashvalue (sat/PH/Day) from on-chain data",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mempool.NewClient(mempoolBase(), httpClient())
			comp, err := usecase.GetHashvalue(c)
			if err != nil {
				return err
			}
			if verbose {
				printResult(formatter.FormatHashvalueVerbose(comp, mempoolBase()))
			} else {
				printResult(formatter.FormatHashvalue(comp))
			}
			return nil
		},
	}

	oceanStats := &cobra.Command{
		Use:   "ocean-account-stats",
		Short: "Fetch Ocean hashrate stats for OCEAN_ADDRESS",
		RunE: func(cmd *cobra.Command, args []string) error {
			addrStr := strings.TrimSpace(os.Getenv("OCEAN_ADDRESS"))
			if addrStr == "" {
				return fmt.Errorf("OCEAN_ADDRESS environment variable is required")
			}
			addr, err := domain.ParseBtcAddress(addrStr)
			if err != nil {
				return fmt.Errorf("invalid OCEAN_ADDRESS: %w", err)
			}
			c := ocean.NewClient(ocean.DefaultOceanURL, httpClient())
			stats, err := usecase.GetOceanAccountStats(c, addr)
			if err != nil {
				return err
			}
			printResult(formatter.FormatOceanStats(stats, addr))
			return nil
		},
	}

	setBids := &cobra.Command{
		Use:   "set-bids",
		Short: "One-shot reconcile bids from a TOML config",
		Long: `Load --bid-config and run a single reconciliation pass.

Config modes (see README):
  • Explicit bids — default; list desired price/speed per [[bids]] row.
  • Target hashrate — mode = "target-hashrate"; plans bids from OCEAN + order book + cooldowns.

If the file contains [watch].enabled = true (timer automation for explicit bids), use
  hashbidder watch --bid-config <same file>
instead; set-bids will refuse that shape so one-shot and loop modes stay distinct.

Requires --bid-config. Uses global --dry-run, -v, --log-file.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			bidConfig, _ := cmd.Flags().GetString("bid-config")
			confAny, err := cfg.LoadConfig(bidConfig)
			if err != nil {
				return err
			}
			bc := braiins.NewClient(braiins.APIBase, braiinsKey(), httpClient())
			oc := ocean.NewClient(ocean.DefaultOceanURL, httpClient())

			switch v := confAny.(type) {
			case cfg.WatchModeConfig:
				return fmt.Errorf("config has [watch].enabled = true: use `hashbidder watch --bid-config %s` for the timer loop, or set watch.enabled = false for one-shot set-bids", bidConfig)
			case cfg.TargetHashrateConfig:
				addrStr := strings.TrimSpace(os.Getenv("OCEAN_ADDRESS"))
				if addrStr == "" {
					return fmt.Errorf("OCEAN_ADDRESS environment variable is required")
				}
				addr, err := domain.ParseBtcAddress(addrStr)
				if err != nil {
					return fmt.Errorf("invalid OCEAN_ADDRESS: %w", err)
				}
				res, err := usecase.SetBidsTarget(bc, oc, addr, v, dryRun, time.Now().UTC())
				if err != nil {
					return err
				}
				if verbose {
					printResult(formatter.FormatSetBidsTargetResultVerbose(res))
				} else {
					printResult(formatter.FormatSetBidsTargetResult(res))
				}
				if res.SetBidsResult.BalanceCheck.Status == domain.BalanceInsufficient {
					os.Exit(1)
				}
				return nil
			case domain.SetBidsConfig:
				res, err := usecase.SetBids(bc, v, dryRun)
				if err != nil {
					return err
				}
				printResult(formatter.FormatSetBidsResult(res))
				if res.BalanceCheck.Status == domain.BalanceInsufficient {
					os.Exit(1)
				}
				return nil
			default:
				return fmt.Errorf("unexpected config type %T", v)
			}
		},
	}
	setBids.Flags().String("bid-config", "", "Path to TOML (explicit bids or target-hashrate; not [watch].enabled)")
	_ = setBids.MarkFlagRequired("bid-config")

	watch := &cobra.Command{
		Use:   "watch",
		Short: "Timer-driven bid reconciliation (explicit TOML + [watch] section)",
		Long: `Run until SIGINT or SIGTERM: sleep between ticks, then re-read the order book and
adjust per-row prices when watch_strategy is set, then reconcile (same engine as set-bids).

Requirements:
  • TOML must be explicit bids (no mode = "target-hashrate").
  • Root table [watch] with enabled = true (interval_seconds, optional jitter_seconds, initial_delay_seconds).
  • At least one [[bids]] row with watch_strategy (e.g. served_floor_band) and price_min / price_max sat/PH/day.

Optional rows omit watch_strategy — they keep the static price from the file.

Examples:
  hashbidder watch --bid-config bids.watch.example.toml --dry-run
  hashbidder watch --bid-config bids.toml -v

See README "Watch mode" and bids.watch.example.toml for all TOML keys.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			bidConfig, _ := cmd.Flags().GetString("bid-config")
			confAny, err := cfg.LoadConfig(bidConfig)
			if err != nil {
				return err
			}
			wm, ok := confAny.(cfg.WatchModeConfig)
			if !ok {
				return fmt.Errorf("watch requires explicit bids plus [watch].enabled = true in %q (got %T)", bidConfig, confAny)
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			c := braiins.NewClient(braiins.APIBase, braiinsKey(), httpClient())
			slog.Info("watch: started", "config", bidConfig, "dry_run", dryRun)
			return watchrun.Run(ctx, c, &wm, dryRun)
		},
	}
	watch.Flags().String("bid-config", "", "Path to TOML with [watch].enabled and explicit [[bids]]")
	_ = watch.MarkFlagRequired("bid-config")

	root.AddCommand(ping, bids, hashvalue, oceanStats, setBids, watch)
	return root
}

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
