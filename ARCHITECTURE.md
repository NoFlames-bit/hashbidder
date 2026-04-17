# Architecture

hashbidder is a **Python 3.13+** command-line tool that talks to **Braiins Hashpower** (spot market), **mempool.space-compatible** APIs (on-chain stats), and **OCEAN** (pool hashrate HTML). It reconciles live spot bids with a TOML config—either **explicit bid rows** or a **target hashrate** mode that plans bids from the order book and OCEAN’s 24h average.

This document describes how the code is layered, how data flows through the main commands, and the design constraints enforced in the repo.

---

## Goals and scope

- **Primary goal:** safely and predictably drive Braiins spot bids toward a declared desired state, with dry-run support, balance checks, and explicit reconciliation plans.
- **Secondary utilities:** connectivity check (`ping`), list bids (`bids`), **hashvalue** (expected sat/PH/day from chain data), and **OCEAN account stats** (read-only).
- **Out of scope:** long-running daemons, web UI, persistence beyond env/config files, and non-spot Braiins products.

---

## Technology stack

| Area | Choice |
|------|--------|
| CLI | [Click](https://click.palletsprojects.io/) — command group, global `-v` / `--log-file`, `ctx.obj` for shared clients |
| HTTP | [httpx](https://www.python-httpx.org/) — synchronous `Client`, 10s timeout on CLI-constructed clients |
| Env | [python-dotenv](https://pypi.org/project/python-dotenv/) — `load_dotenv()` at CLI startup |
| Config files | stdlib `tomllib` — TOML for bid configs |
| Packaging | `uv` + `pyproject.toml` / Hatchling; entry point `hashbidder = hashbidder.main:main` |
| Quality | Ruff (format + lint), Mypy strict, pytest, **import-linter** architectural contracts |

---

## Layered architecture

The codebase follows a **ports-and-adapters** style without heavy ceremony: **domain** types and pure logic sit at the center; **use cases** orchestrate; **clients** implement HTTP boundaries; **main** wires I/O and translates errors to user-facing CLI messages.

```mermaid
flowchart TB
  subgraph cli["CLI (`hashbidder/main.py`)"]
    Click[Click commands]
    ClientsCtx["`Clients` dataclass: braiins, mempool, ocean"]
    ErrMap["`_api_errors` / `_mempool_errors` / `_ocean_errors`"]
  end

  subgraph usecases["Use cases (`hashbidder/use_cases/`)"]
    ping_uc[ping / get_current_bids]
    hv_uc[get_hashvalue]
    ocean_uc[get_ocean_account_stats]
    sb_uc[set_bids]
    sbt_uc[set_bids_target]
  end

  subgraph infra["Infrastructure"]
    Braiins["`BraiinsClient` — `HashpowerClient` protocol"]
    Mempool["`MempoolClient` — `MempoolSource` protocol"]
    Ocean["`OceanClient` — `OceanSource` protocol"]
  end

  subgraph core["Core orchestration"]
    Runner["`bid_runner.reconcile` / `execute_plan`"]
    Planner["`domain.bid_planning.plan_bid_changes`"]
    Balance["`domain.balance_check.check_balance`"]
    Target["`target_hashrate` + `hashvalue.compute_hashvalue`"]
  end

  subgraph domain["Domain (`hashbidder/domain/`)"]
    Types[Value types: hashrate, sats, upstream, bids, ...]
  end

  Click --> ClientsCtx
  Click --> usecases
  usecases --> Braiins
  usecases --> Mempool
  usecases --> Ocean
  sb_uc --> Runner
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

### Domain (`hashbidder/domain/`)

**No imports** from `client`, `main`, `use_cases`, `config`, `bid_runner`, or other outer packages—this is **enforced by import-linter** (`pyproject.toml`).

Responsibilities:

- **Money and units:** `Sats`, `Hashrate`, `HashratePrice`, `HashUnit`, `TimeUnit`, `PriceTick`, `SatsBurnRate`.
- **Market concepts:** `Upstream`, `StratumUrl`, `BidConfig`, `SetBidsConfig`, `UserBid`, `BidStatus`, `Progress`.
- **Pure planning:** `bid_planning.plan_bid_changes` — greedy matching of live bids to config slots, emitting **cancels**, **edits**, **creates**, and **unchanged**; upstream mismatch → cancel + create (upstream cannot be edited in place).
- **Risk gate:** `balance_check.check_balance` — sums create `amount_sat` vs available balance; derives burn rate from planned creates and flags **LOW** runway (below 72h) vs **SUFFICIENT** / **INSUFFICIENT**.
- **Bitcoin helpers:** e.g. `block_subsidy`, `bitcoin` constants used by hashvalue.

### Use cases (`hashbidder/use_cases/`)

Thin **application services**: fetch data, call domain/pure functions, delegate to `bid_runner` where appropriate.

- **Independence:** import-linter requires **no cross-imports** between `hashvalue`, `ocean`, `ping`, `set_bids`, and `set_bids_target` modules.
- **No CLI:** `hashbidder.main` is forbidden from `use_cases` (keeps Click out of business logic).

`set_bids` is a one-liner wrapper around `reconcile`. `set_bids_target` composes OCEAN stats, order book, market settings, cooldown-aware bid building (`target_hashrate`), then calls `reconcile` with the computed `SetBidsConfig`.

### Bid runner (`hashbidder/bid_runner.py`)

The **reconciliation engine** for explicit (or computed) `SetBidsConfig`:

1. `get_current_bids` → `plan_bid_changes` → `check_balance`.
2. If `dry_run` or balance **INSUFFICIENT**, return plan + balance only (**no API mutations** on insufficient balance).
3. Otherwise `execute_plan`: **cancels → edits → creates**, each action retried up to **3** times on transient `ApiError` (429 / 5xx) with **5s** delay between attempts.
4. Special case: if an **upstream-mismatch** cancel fails, the linked **create is skipped** to avoid duplicate exposure.
5. After mutations, sleeps **`POST_EXECUTE_REFETCH_DELAY_SECONDS` (3s)** before refetching bids—documented workaround for Braiins **stale `/spot/bid/current`** briefly after writes.

### Clients (`hashbidder/client.py`, `mempool_client.py`, `ocean_client.py`)

- **`HashpowerClient`** (protocol) and **`BraiinsClient`** — Braiins Hashpower v1 base URL `https://hashpower.braiins.com/v1`; maps JSON ↔ domain types; `ApiError` with `is_transient` for retries; grpc-message header decoding on errors.
- **`MempoolSource`** / **`MempoolClient`** — default base `https://mempool.bitcoinbarcelona.xyz` (override with `MEMPOOL_URL`). `get_chain_stats` uses a **two-call consistent tip** strategy (reward-stats `endBlock` + block difficulty) documented in code to avoid height/fee skew across requests.
- **`OceanSource`** / **`OceanClient`** — default `https://ocean.xyz`; fetches HTML fragment and **regex-parses** fixed table shape; fragile by design if OCEAN changes markup (tests and errors surface schema drift).

### Config loading (`hashbidder/config.py`)

Parses TOML into either:

- **`SetBidsConfig`** — default sats per create, upstream, and `tuple[BidConfig, ...]` from `[[bids]]` (explicit mode; default `mode` is explicit).
- **`TargetHashrateConfig`** — `mode = "target-hashrate"`, `target_hashrate_ph_s`, `max_bids_count`, no `[[bids]]` allowed.

Shared validation: `default_amount_sat`, `[upstream]` with `url` + `identity` (`StratumUrl` validation).

### CLI (`hashbidder/main.py`)

- Constructs **`Clients`** once per invocation on `ctx.obj`: `BraiinsClient` (API key from `BRAIINS_API_KEY`), `MempoolClient`, `OceanClient`.
- **Commands:** `ping`, `bids`, `hashvalue`, `ocean-account-stats`, `set-bids`.
- **`set-bids`:** loads config; branches on `TargetHashrateConfig` vs `SetBidsConfig`; target mode requires **`OCEAN_ADDRESS`** (validated `BtcAddress`). Exits **1** if post-run balance check is **INSUFFICIENT** (explicit and target paths).
- **Logging:** stderr console; optional file; DEBUG when `-v`.

### Presentation (`hashbidder/formatting.py`)

String builders for human-readable CLI output (plans, execution outcomes, hashvalue verbose, OCEAN stats, target-mode verbose trace). Keeps Click/`echo` out of domain.

### Target-hashrate planning (`hashbidder/target_hashrate.py`)

Pure (and mostly testable) logic that **does** depend on **`OrderBook` / `UserBid` / `MarketSettings`** types from `client.py` — a deliberate coupling so “cheapest served bid” and cooldown metadata use the same shapes as the API layer.

Highlights:

- **`compute_needed_hashrate`** — closed-form adjustment so adding `needed` for the **next 12 hours** moves the rolling **24h** average toward `target`; clamped at zero if already at/above target.
- **`distribute_bids`** — splits PH/s across up to `max_bids_count` bids, minimum granularity rules (e.g. below 0.5 PH/s → no bids; below 1 PH/s → single 1 PH/s bid).
- **`check_cooldowns` / `plan_with_cooldowns`** — respects Braiins **minimum decrease periods** for price and speed from `MarketSettings`; locks fields that cannot be lowered yet while still allowing increases.
- **`find_market_price`** — lowest **served** order-book bid (positive `hr_matched_ph`), aligned to tick, then **undercut by one tick** from above.

### Hashvalue (`hashbidder/hashvalue.py` + use case)

Computes **expected sats per PH per day** from difficulty, epoch fee total, and subsidy logic (`BLOCKS_PER_EPOCH` window from mempool). Independent of Braiins trading.

---

## External systems and configuration

| Variable / input | Purpose |
|------------------|---------|
| `BRAIINS_API_KEY` | Authenticated Hashpower calls; owner key required for mutations |
| `MEMPOOL_URL` | Optional override for mempool API base |
| `OCEAN_ADDRESS` | Bitcoin payout address for OCEAN stats and target-hashrate mode |

CLI flags: `--bid-config` + `--dry-run` for `set-bids`; global `-v`, `--log-file`.

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

---

## Error handling philosophy

- **CLI boundary:** HTTP, timeout, and domain `ValueError` issues are caught in context managers and rethrown as **`click.ClickException`** with readable messages (`_api_errors`, `_mempool_errors`, `_ocean_errors`).
- **API layer:** Braiins uses **`ApiError`** with status + decoded message; mempool/ocean use their own error types.
- **Balance:** insufficient balance **aborts the entire reconcile** before any cancel/edit/create — conservative safety property.

---

## Architectural enforcement (import-linter)

Defined in `pyproject.toml`:

1. **`hashbidder.domain`** must not depend on outward modules (pure core).
2. **`hashbidder.use_cases`** must not import **`hashbidder.main`**.
3. **Use case modules** are pairwise **independent** (no mutual imports).

CI (`.github/workflows/ci.yml`) runs: `ruff format/check`, `mypy`, `uv lock --check`, `lint-imports`, `pytest`.

---

## Extension points

- **Fake / test doubles:** protocols `HashpowerClient`, `MempoolSource`, `OceanSource` allow substituting clients in tests without HTTP.
- **New commands:** add a use case module (respecting import-linter), wire in `main.py`, reuse `Clients` or extend the dataclass if a new port is needed.
- **New config modes:** extend `config.load_config` and a dedicated use case or branch in `set-bids`, then produce a `SetBidsConfig` for `reconcile` whenever possible — keeps execution semantics in one place (`bid_runner`).

---

## Known operational constraints (encoded in design)

- Braiins **tick size** and **cooldowns** are fetched from `/spot/settings` for target mode; explicit configs must use valid tick multiples (README notes current market tick).
- OCEAN integration is **HTML scraping** — operational fragility is accepted for simplicity; parsing is strict (row/cell counts, labels).
- Reconciliation matching is **greedy by “fewest field diffs”** and **amount-remaining sort** — deterministic but not globally optimal; documented here so operators know bids may reorder across runs when multiple similar bids exist.

---

## Repository map (concise)

| Path | Role |
|------|------|
| `hashbidder/main.py` | CLI entry, client wiring, error mapping |
| `hashbidder/use_cases/` | Per-command orchestration |
| `hashbidder/bid_runner.py` | Reconcile + execute + retries + post-mutation delay |
| `hashbidder/domain/` | Types + pure planning + balance semantics |
| `hashbidder/client.py` | Braiins HTTP + `HashpowerClient` protocol |
| `hashbidder/config.py` | TOML → config dataclasses |
| `hashbidder/target_hashrate.py` | Target-mode pricing, need, cooldown, distribution |
| `hashbidder/hashvalue.py` | On-chain hashvalue math |
| `hashbidder/mempool_client.py`, `ocean_client.py` | Secondary HTTP ports |
| `hashbidder/formatting.py` | Human-readable output |
| `tests/` | Unit + CLI tests |

This should be enough for a new contributor to trace any user-facing behavior from the CLI down to HTTP or pure functions without spelunking the whole tree.
