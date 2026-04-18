# hashbidder (Go)

A small CLI that reconciles your bids on the [Braiins Hashpower](https://academy.braiins.com/en/braiins-hashpower/about/) spot market with a TOML config. It calls the [Hashpower API](https://hashpower.braiins.com/api/) to read the order book, compare it to what you want, and create, edit, or cancel bids as needed. TOML and reconciliation behavior match the widely used hashbidder semantics (effective upstream, sibling retention, balance gating).

For how this **Go** codebase is layered, see [ARCHITECTURE.md](ARCHITECTURE.md).

## Disclaimers

This tool is lightly tested and may contain defects. If you aim it at the live Braiins Hashpower market, it spends real funds in a real order book, so you can lose money or lock hashrate in ways you did not intend. **You use it at your own risk.**

The tool is oriented toward miners on [OCEAN](https://ocean.xyz/) who run their own [DATUM gateway](https://github.com/OCEAN-xyz/datum_gateway). Other setups may still work, but some features (especially target-hashrate mode) assume that profile.

## Prerequisites

- **Go 1.26+** (see `go.mod` and `goproject.toml`).
- Optional: **golangci-lint** for `make lint` / `make check`.

## Build

From this directory (the folder that contains `go.mod`):

```sh
make build
# binary: ./bin/hashbidder
```

Or without Make:

```sh
go build -o bin/hashbidder ./cmd/hashbidder
```

## Configuration

### Environment variables

Copy the example file in **this** directory and edit values:

```sh
cp .env.example .env
```

| Variable | Required for | Notes |
|----------|----------------|-------|
| `BRAIINS_API_KEY` | `bids`, `set-bids` | Omit or use a read-only key for read-only API access. Use an **owner** key to place or change bids. |
| `OCEAN_ADDRESS` | `set-bids` in **target-hashrate** mode, `ocean-account-stats` | Your Bitcoin payout address as seen by OCEAN (used to pull 24h hashrate). |
| `MEMPOOL_URL` | Optional | Base URL for the Mempool instance used by `hashvalue` (see `.env.example` for a default). |

`ping` and `hashvalue` do not require a Braiins API key.

### Bid config file (`set-bids`)

`set-bids` reads a TOML file. Two modes are supported:

1. **Explicit bids** — you list each desired bid. Omit `mode`, or set `mode = "explicit-bids"`.
2. **Target hashrate** — you set a goal hashrate; the tool plans bids from the live book and your OCEAN stats. Set `mode = "target-hashrate"`.

#### Explicit bids

Each `[[bids]]` table describes one desired bid. Prices in the file are **sat per PH per day** (`price_sat_per_ph_day`). The exchange enforces a **minimum tick in sat per EH per day**; values you enter must align to that grid when converted (see `PriceTick` in the codebase). The live tick size is also returned from Braiins settings (commonly 1000 sat/EH/Day).

```toml
# Collateral per new bid (sat). If you run reconciliation often, smaller amounts may be enough.
default_amount_sat = 100000

# Where purchased hashrate is delivered (your stratum endpoint and worker name).
[upstream]
url = "stratum+tcp://203.0.113.10:23334"
identity = "rig.worker"

[[bids]]
price_sat_per_ph_day = 45000   # max price you are willing to pay (sat/PH/Day)
speed_limit_ph_s = 1.0         # cap on hashrate for this bid (PH/s)

[[bids]]
price_sat_per_ph_day = 46000
speed_limit_ph_s = 1.0

[[bids]]
price_sat_per_ph_day = 46000
speed_limit_ph_s = 2.0
```

#### One stratum URL, multiple workers

Each Braiins order delivers hashrate to a **dest_upstream**: the same stratum **host and port** as your `[upstream].url`, plus a **worker identity** string. If you run several workers through one DATUM gateway, you usually want **one `[upstream].url`** and a **different `identity=` on each `[[bids]]` row** so each row lines up with `dest_upstream.identity` from the API.

- You may **omit `[upstream].identity`** when **every** `[[bids]]` row sets **`identity=`** (per-row delivery worker).
- If **`[upstream].identity`** is set, it is the **default** for rows that omit `identity=`.

Copy and edit the checked-in example: [bids.multiple-workers.example.toml](bids.multiple-workers.example.toml).

Use the exact header `[[bids]]` (all lowercase) for every worker row. Mixed spellings such as `[[Bids]]` are normalized when the file is loaded so each table stays a distinct row.

#### How `set-bids` reconciles open bids (operating model)

- **Effective upstream** — A config row’s delivery target is `[upstream].url` plus either that row’s `identity=` or, if empty, `[upstream].identity`. Matching compares **trimmed** identity strings and the stratum **host:port** only (**`stratum+tcp` vs `stratum+ssl` does not matter** for “same pool”).
- **Greedy matching** — Each `[[bids]]` row is paired with at most one manageable live bid (same effective upstream), preferring the fewest price/speed field differences, then higher remaining collateral.
- **Surplus orders you still declare** — If your file lists an identity more than once (duplicate rows) or there are extra live bids for an identity that appears in the file but no row consumed them, those extras are **canceled** as unmatched managed workers.
- **Sibling workers not in the file** — Manageable live bids on the **same** stratum URL as `[upstream]` whose identity **does not** appear on any `[[bids]]` line are **left alone**, so you can add workers incrementally without canceling unrelated orders.
- **Upstream mismatch** — Changing stratum URL or identity on an existing order is not an in-place edit on Braiins; the plan uses **cancel + create**.
- **Empty desired state** — A file with **`default_amount_sat` and `[upstream]` but no `[[bids]]` tables** means “cancel every manageable bid.” **`[upstream].identity` may be omitted** in that case. As soon as you add **`[[bids]]` rows** without a default `[upstream].identity`, **each row must set `identity=`**.

#### Target hashrate mode

Declare a target **PH/s** and a maximum number of parallel bids. The tool reads your rolling OCEAN hashrate, derives how much more capacity you need, chooses a price by undercutting the cheapest filled bid on the book by one tick (see Braiins docs on [cooldowns / overbid](https://academy.braiins.com/en/braiins-hashpower/faqs/trading/?Pages_en%5Bquery%5D=cooldow#what-is-the-overbid-feature)), and splits the remainder across up to `max_bids_count` bids while respecting per-bid cooldown rules.

```toml
mode = "target-hashrate"

# Collateral per new bid (sat). Filled orders are replaced on the next run if the planner still needs capacity.
default_amount_sat = 100000

target_hashrate_ph_s = 5.0

# Parallel bids give the planner room to work around Braiins cooldowns. If you run every ~10 minutes, try starting around 5; less frequent runs may need fewer slots.
max_bids_count = 5

[upstream]
url = "stratum+tcp://203.0.113.10:23334"
identity = "rig.worker"
```

Target mode does **not** allow `[[bids]]` sections in the same file.

## Usage

After `make build`:

```sh
./bin/hashbidder --help
```

Or one-off without installing a binary:

```sh
go run ./cmd/hashbidder --help
```

### Global options

- **`-v` / `--verbose`** — debug logging; for `set-bids` in target-hashrate mode, also prints planner detail.
- **`--log-file PATH`** — tee logs to a file (with a short startup marker when set).
- **`--dry-run`** — for `set-bids`, print the plan only (no API mutations except reads).

## Commands

Illustrative output only; real numbers depend on the market and your account.

```sh
./bin/hashbidder ping
./bin/hashbidder bids
./bin/hashbidder hashvalue
./bin/hashbidder ocean-account-stats
./bin/hashbidder set-bids --bid-config bids.toml --dry-run
```

If the account balance check fails after planning, `set-bids` exits with status **1** (explicit and target paths).

## Development

```sh
make check    # fmt, vet, lint, race + coverage tests
make test     # go test ./...
```

All commands above assume your current working directory is the one containing `go.mod` (this module).
