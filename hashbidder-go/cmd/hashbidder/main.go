package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/braiins"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/cfg"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/formatter"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/mempool"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/ocean"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/usecase"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var (
	verbose bool
	logFile string
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
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			_ = godotenv.Load()
			setupLogging()
		},
	}

	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable debug logging")
	root.PersistentFlags().StringVar(&logFile, "log-file", "", "Also log to this file")

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
			fmt.Printf("OK — order book: %d bids, %d asks\n", len(book.Bids), len(book.Asks))
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
				fmt.Println("No active bids.")
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
				fmt.Printf("%s  %14s  price=%s  limit=%s  remaining=%s sat  progress=%s\n",
					bid.ID, bid.Status, price.String(), bid.SpeedLimitPH.String(), rem, prog)
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
				fmt.Println(formatter.FormatHashvalueVerbose(comp, mempoolBase()))
			} else {
				fmt.Println(formatter.FormatHashvalue(comp))
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
			fmt.Println(formatter.FormatOceanStats(stats, addr))
			return nil
		},
	}

	setBids := &cobra.Command{
		Use:   "set-bids",
		Short: "Reconcile bids to a TOML config",
		RunE: func(cmd *cobra.Command, args []string) error {
			bidConfig, _ := cmd.Flags().GetString("bid-config")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			confAny, err := cfg.LoadConfig(bidConfig)
			if err != nil {
				return err
			}
			bc := braiins.NewClient(braiins.APIBase, braiinsKey(), httpClient())
			oc := ocean.NewClient(ocean.DefaultOceanURL, httpClient())

			switch v := confAny.(type) {
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
					fmt.Println(formatter.FormatSetBidsTargetResultVerbose(res))
				} else {
					fmt.Println(formatter.FormatSetBidsTargetResult(res))
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
				fmt.Println(formatter.FormatSetBidsResult(res))
				if res.BalanceCheck.Status == domain.BalanceInsufficient {
					os.Exit(1)
				}
				return nil
			default:
				return fmt.Errorf("unexpected config type %T", v)
			}
		},
	}
	setBids.Flags().String("bid-config", "", "Path to the TOML bid config file")
	_ = setBids.MarkFlagRequired("bid-config")
	setBids.Flags().Bool("dry-run", false, "Print what would change without executing")

	root.AddCommand(ping, bids, hashvalue, oceanStats, setBids)
	return root
}

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
