# Architecture (Go)

hashbidder-go is a **Go 1.26+** CLI that talks to **Braiins Hashpower** (spot market), **mempool.space-compatible** APIs (on-chain stats), and **OCEAN** (pool hashrate HTML). It reconciles live spot bids with a TOML config—either **explicit bid rows** or **target hashrate** mode—with reconciliation semantics documented in this repo’s [README.md](README.md) (multi-worker identities, stratum URL matching, sibling retention, balance gating, cancel/edit/create ordering).

This document describes how the **Go** code is organized, how data flows through commands, and where behavior is implemented. For operator-facing TOML examples and disclaimers, see [README.md](README.md).

---

## Goals and scope

- **Primary goal:** safely drive Braiins spot bids toward a declared desired state, with dry-run support, balance checks, and explicit reconciliation plans.
- **Secondary utilities:** connectivity (`ping`), list bids (`bids`), **hashvalue** (expected sat/PH/day from chain data), and **OCEAN account stats** (read-only).
- **Out of scope:** long-running daemons, web UI, persistence beyond env/config files, and non-spot Braiins products.

---

## Technology stack

| Area | Choice |
|------|--------|
| CLI | [Cobra](https://github.com/spf13/cobra) — command tree, persistent `-v` / `--log-file`, global `--dry-run` for `set-bids` |
| HTTP | `net/http` — `Client` with **10s** timeout constructed in `cmd/hashbidder` |
| Env | [godotenv](https://github.com/joho/godotenv) — `.env` loaded in `PersistentPreRun` |
| Config | [go-toml v2](https://github.com/pelletier/go-toml) + `shopspring/decimal` for numeric TOML fields |
| Module | `go.mod` in this directory; **`go.work`** with `use .` anchors the toolchain here when the editor workspace is wider; build via `go build ./cmd/hashbidder` or `make build` |
| Quality | `go fmt`, `go vet`, **golangci-lint** (`make lint`), `go test` (`Makefile` targets including race + coverage) |

---

## Layered architecture

Layout mirrors the Python port: **domain** at the center, **use cases** orchestrating, **HTTP clients** at the edges, **`cmd/hashbidder`** wiring I/O and logging.

```mermaid
flowchart TB
  subgraph cli["CLI (`cmd/hashbidder/main.go`)"]
    Cobra[Cobra commands]
    Clients["Braiins / Mempool / Ocean clients per command"]
    Log[slog + optional tee to file]
  end

  subgraph usecases["Use cases (`internal/usecase/`)"]
    ping_uc[implicit in main: order book / bids]
    hv_uc[GetHashvalue]
    ocean_uc[GetOceanAccountStats]
    sb_uc[SetBids]
    sbt_uc[SetBidsTarget]
    watch_uc["watch command → `internal/watchrun` → SetBids"]
  end

  subgraph infra["Infrastructure"]
    Braiins["`internal/braiins` — `HashpowerClient` interface"]
    Mempool["`internal/mempool` — `Source` interface"]
    Ocean["`internal/ocean` — HTML client"]
  end

  subgraph core["Core orchestration"]
    Runner["`internal/bidrunner` — Reconcile / ExecutePlan"]
    Planner["`internal/domain` — PlanBidChanges"]
    Balance["`internal/domain` — CheckBalance"]
    Target["`internal/targethr` + `internal/hv`"]
  end

  subgraph domain["Domain (`internal/domain/`)"]
    Types[Value types: hashrate, sats, upstream, bids, ...]
  end

  Cobra --> Clients
  Cobra --> usecases
  usecases --> Braiins
  usecases --> Mempool
  usecases --> Ocean
  sb_uc --> Runner
  watch_uc --> Runner
  sbt_uc --> Target
  sbt_uc --> Runner
  Runner --> Planner
  Runner --> Balance
  hv_uc --> Target
  Braiins --> domain
  Mempool --> domain
  Ocean --> domain
  Planner --> domain
```

### Domain (`internal/domain/`)

Pure types and planning logic. Packages **outside** `domain` import it; `domain` does not import `braiins`, `bidrunner`, `usecase`, `cfg`, or `cmd`.

Responsibilities:

- **Money and units:** `Sats`, `Hashrate`, `HashratePrice`, hash/time units, `PriceTick`, `SatsBurnRate`.
- **Market concepts:** `Upstream`, `StratumUrl`, bid config types, `UserBid`, statuses, `Progress`.
- **Bid timeline:** `BidHistory` / `BidHistoryEntry` — normalised newest-first history from the bid-detail API; strict price/speed decrease timestamps for cooldown logic.
- **Pure planning:** `PlanBidChanges` — same greedy matching rules as Python: effective upstream (host:port + trimmed identity; scheme-insensitive), cancels/edits/creates/unchanged, upstream mismatch → cancel + create, sibling retention for identities not listed when `[[bids]]` is non-empty, full cancel when there are no bid rows.
- **Risk gate:** `CheckBalance` — create collateral vs available balance; LOW runway vs SUFFICIENT / INSUFFICIENT (aligned with Python thresholds).
- **Bitcoin helpers:** subsidy, constants used by hashvalue paths.

### Use cases (`internal/usecase/`)

Thin orchestration: fetch via clients, call domain and `bidrunner`, return structured results for formatters.

- `set_bids.go` — loads `domain.SetBidsConfig` (already parsed), calls `bidrunner.Reconcile`.
- `target.go` — OCEAN stats + order book + settings; builds config via `internal/targethr`, then reconciles.

### Bid runner (`internal/bidrunner/`)

Reconciliation engine for explicit (or computed) `domain.SetBidsConfig`:

1. `GetCurrentBids` → `PlanBidChanges` → `CheckBalance`.
2. If `dry_run` or balance **insufficient**, return plan + balance only (**no mutations**).
3. Otherwise `ExecutePlan`: **cancels → edits → creates**, each action retried up to **3** times on transient API errors (**429 / 5xx**) with **5s** delay (injectable `sleep` for tests).
4. If an **upstream-mismatch** **cancel** fails, the linked **create is skipped**.
5. After any mutation batch, sleeps **`postExecuteRefetchDelay` (3s)** before refetching bids (Braiins stale read workaround).

`Reconcile` logs **deferred creates** when a delivery slot is blocked by a non-manageable bid status (operational visibility).

### Clients

- **`internal/braiins`** — Hashpower v1 base URL; JSON ↔ domain; `APIError` with `IsTransient()`; implements `HashpowerClient` for production and tests (including `GetBidHistory` → `GET /spot/bid/detail/{id}`).
- **`internal/mempool`** — default base `DefaultMempoolURL`; two-call consistent tip for chain stats (same idea as Python).
- **`internal/ocean`** — fetches HTML and regex-parses the stats table; same fragility contract as Python.

### Config loading (`internal/cfg/`)

Parses TOML into **`domain.SetBidsConfig`** (explicit / default), **`cfg.TargetHashrateConfig`**, or **`cfg.WatchModeConfig`** when **`[watch].enabled = true`** on an explicit-bids file (timer automation: loop settings + per-row `BidWatchRule` values). Validates upstream URL, amounts, and mode-specific rules (e.g. `[upstream].identity` required in target mode; per-row `identity` rules in explicit mode; **`[watch]`** is rejected together with target-hashrate mode).

**Header normalization:** before decode, every root-level **`[[…bids…]]`** array-of-tables header is rewritten to **`[[bids]]`**. This avoids go-toml collapsing mixed-case bid tables into a single row.

### CLI (`cmd/hashbidder/main.go`)

- Loads `.env`, configures **slog** (stderr; optional tee to `--log-file`).
- **Commands:** `ping`, `bids`, `hashvalue`, `ocean-account-stats`, `set-bids --bid-config`, **`watch --bid-config`**.
- **`set-bids`:** `cfg.LoadConfig`; branch on `WatchModeConfig` (error: use `watch`) vs `TargetHashrateConfig` vs `domain.SetBidsConfig`; target mode requires **`OCEAN_ADDRESS`**. Exits **1** if post-run balance status is insufficient (matches Python).
- **`watch`:** same loader; requires `WatchModeConfig`. Runs **`internal/watchrun`** until SIGINT/SIGTERM: sleep (`interval_seconds` + optional jitter), optional per-row price strategy, then **`usecase.SetBids`** (same reconcile as one-shot). Global **`--dry-run`** applies each tick. **SIGHUP** reloads the **`--bid-config`** path and runs **`usecase.SetBids`** once for the new file; on failure the previous `WatchModeConfig` is retained (see README “Reloading the TOML”).
- **Verbose:** debug logs; target mode also prints planner detail via `formatter` verbose helpers.

### Presentation (`internal/formatter/`)

String builders for CLI output (plans, execution, hashvalue verbose, OCEAN stats, target-mode traces). Keeps Cobra/`main` free of long format strings where possible.

### Target hashrate (`internal/targethr/`)

Planning for target mode: need from rolling 24h average, distribution across bid slots, cooldown-aware field locks, market price scan (undercut cheapest **served** bid by one tick). Cooldowns use **tier-1** predicates on `UserBid.LastUpdated` when both fields are provably past their decrease windows; otherwise **`GetBidHistory`** plus `domain.BidHistory` for authoritative per-field flags, with a conservative fallback on `*APIError` from history (matches Python PR #17). Uses Braiins order-book and settings shapes from `braiins` (same coupling idea as Python’s `target_hashrate` ↔ client types).

### Watch loop (`internal/watchrun/`)

Optional **explicit-bids** automation: **`ResolveCooldowns`** each tick (order book + settings + bid history), **`served_floor_band`** strategy adjusts selected rows’ desired prices toward the served-stack undercut clamped to per-row min/max, then **`usecase.SetBids`**. A goroutine forwards **SIGHUP** into the main loop so reload can interrupt sleep or initial delay; reload uses **`cfg.LoadConfig`** plus a full **`SetBids`** pass before swapping in-memory config. Not a daemon inside `bidrunner`; it is a separate command and package so CLI and cron-style wrappers stay thin.

### Hashvalue (`internal/hv/` + use case)

Computes expected sats per PH per day from mempool chain stats and domain subsidy/fee window logic.

---

## External systems and configuration

| Variable / input | Purpose |
|------------------|---------|
| `BRAIINS_API_KEY` | Authenticated Hashpower calls |
| `MEMPOOL_URL` | Optional mempool API base override |
| `OCEAN_ADDRESS` | Payout address for OCEAN stats and target-hashrate mode |

Flags: `--bid-config` for `set-bids` and **`watch`**; global `-v`, `--log-file`, `--dry-run`.

---

## Command → dependency map

| Command | Braiins | Mempool | OCEAN |
|---------|---------|---------|-------|
| `ping` | order book | — | — |
| `bids` | current bids | — | — |
| `hashvalue` | — | chain stats | — |
| `ocean-account-stats` | — | — | HTML stats |
| `set-bids` (explicit) | full reconcile | — | — |
| `set-bids` (target) | reconcile + settings + order book | — | 24h hashrate |
| `watch` | reconcile + settings + order book each tick; bid detail when cooldowns need history | — | — |

---

## Error handling philosophy

- **CLI boundary:** `RunE` returns `error`; `main` prints message and exits **1**.
- **API layer:** Braiins `APIError`; mempool `MempoolError`; ocean returns wrapped errors from HTTP/parse failures.
- **Balance:** insufficient balance aborts reconcile **before** any cancel/edit/create.

---

## Architectural enforcement

Go does not use import-linter-style contracts; discipline is **package boundaries** and review. `internal/` hides implementation from external importers of the module. Local quality gates are the **Makefile** targets (`check` = fmt, vet, lint, test with race + coverage); wire them into CI however you clone this tree.

---

## Extension points

- **Test doubles:** `braiins.HashpowerClient` and fakes in `internal/testutil` / tests.
- **New commands:** add Cobra `RunE`, optional new `internal/usecase` func, reuse clients.
- **New config modes:** extend `cfg.LoadConfig` / `WatchModeConfig` and strategies under `internal/watchrun/`, or add branches in `set-bids` / `watch` as needed.

---

## Known operational constraints

- Braiins tick size and cooldowns from `/spot/settings` in target mode.
- OCEAN HTML scraping fragility.
- Greedy reconciliation is deterministic but not globally optimal.
- Multi-worker explicit configs retain sibling identities not listed in the file.

---

## Repository map (Go)

| Path | Role |
|------|------|
| `cmd/hashbidder/main.go` | CLI entry, client wiring, slog |
| `internal/usecase/` | Per-command orchestration |
| `internal/bidrunner/` | Reconcile, execute, retries, post-mutation delay |
| `internal/domain/` | Types, planning, balance |
| `internal/braiins/` | Braiins HTTP + `HashpowerClient` |
| `internal/cfg/` | TOML → configs |
| `internal/targethr/` | Target-mode planning |
| `internal/watchrun/` | `watch` command: timer loop, strategies, per-tick reconcile |
| `internal/hv/` | On-chain hashvalue math |
| `internal/mempool/`, `internal/ocean/` | Secondary HTTP |
| `internal/formatter/` | Human-readable output |
| `Makefile` | build, test, lint, check |
| `bids.multiple-workers.example.toml` | Example: one `[upstream].url`, per-row `identity=` |
| `bids.watch.example.toml` | Example: `[watch]` + per-row `watch_strategy` for `hashbidder watch` |

This should be enough to trace any Go CLI behavior from `cmd/` down to HTTP or pure `domain` code.
