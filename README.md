# hashbidder

hashbidder is a small CLI that reconciles your bids on the [Braiins Hashpower](https://academy.braiins.com/en/braiins-hashpower/about/) spot market with a TOML config file. It calls the [Hashpower API](https://hashpower.braiins.com/api/) to read the order book, compare it to what you want, and create, edit, or cancel bids as needed.

For layers, data flow, and design decisions, see [ARCHITECTURE.md](ARCHITECTURE.md).

## Disclaimers

hashbidder is lightly tested and may contain defects. If you aim it at the live Braiins Hashpower market, it spends real funds in a real order book, so you can lose money or lock hashrate in ways you did not intend. **You use hashbidder at your own risk.**

The tool is oriented toward miners on [OCEAN](https://ocean.xyz/) who run their own [DATUM gateway](https://github.com/OCEAN-xyz/datum_gateway). Other setups may still work, but some features (especially target-hashrate mode) assume that profile.

## Prerequisites

- **Python 3.13+** (see `requires-python` in `pyproject.toml`).
- **[uv](https://docs.astral.sh/uv/getting-started/installation/)** for running the CLI and dev tasks.

## Configuration

### Environment variables

Copy the example file and edit values as needed:

```sh
cp .env.example .env
```

| Variable | Required for | Notes |
|----------|----------------|-------|
| `BRAIINS_API_KEY` | `bids`, `set-bids` | Omit or use a read-only key for read-only API access. Use an **owner** key if you want hashbidder to place or change bids. |
| `OCEAN_ADDRESS` | `set-bids` in **target-hashrate** mode, `ocean-account-stats` | Your Bitcoin payout address as seen by OCEAN (used to pull 24h hashrate). |
| `MEMPOOL_URL` | Optional | Base URL for the Mempool instance used by `hashvalue` (see `.env.example` for a default). |

`ping` and `hashvalue` do not require a Braiins API key.

### Bid config file (`set-bids`)

`set-bids` reads a TOML file. Two modes are supported:

1. **Explicit bids** — you list each desired bid. Omit `mode`, or set `mode = "explicit-bids"`.
2. **Target hashrate** — you set a goal hashrate; hashbidder plans bids from the live book and your OCEAN stats. Set `mode = "target-hashrate"`.

Start from one of the examples below and adjust.

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

#### Target hashrate mode

Declare a target **PH/s** and a maximum number of parallel bids. hashbidder reads your rolling OCEAN hashrate, derives how much more capacity you need, chooses a price by undercutting the cheapest filled bid on the book by one tick (see Braiins docs on [cooldowns / overbid](https://academy.braiins.com/en/braiins-hashpower/faqs/trading/?Pages_en%5Bquery%5D=cooldow#what-is-the-overbid-feature)), and splits the remainder across up to `max_bids_count` bids while respecting per-bid cooldown rules.

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

From the repository root:

```sh
uv run hashbidder --help
```

On first run, `uv` creates a virtual environment and installs dependencies.

### Global options

- **`-v` / `--verbose`** — debug logging; for `set-bids` in target-hashrate mode, also prints planner detail (price scan, distribution, cooldown status).
- **`--log-file PATH`** — append the same logs to a file.

## Commands

Illustrative output only; real numbers depend on the market and your account.

```sh
# Public order book (no API key required)
$ uv run hashbidder ping
OK — order book: 70 bids, 8 asks

# List your active bids (requires owner or read-capable key as appropriate)
$ uv run hashbidder bids
B123456789          ACTIVE  price=500 sat/PH/Day  limit=5.0 PH/Second  remaining=100000 sat  progress=0.42

# Implied hashprice from chain data via Mempool (no Braiins key)
$ uv run hashbidder hashvalue
Hashvalue: 45469 sat/PH/Day

# OCEAN hashrate windows for OCEAN_ADDRESS (debugging / before target-hashrate)
$ uv run hashbidder ocean-account-stats
Ocean stats for bc1qxy2k…

    24 hrs    4.12 PH/s
     3 hrs    4.08 PH/s
    10 min    4.15 PH/s

# Reconcile open bids to the config; --dry-run prints the plan only.
$ uv run hashbidder set-bids --bid-config bids.toml --dry-run
=== Account Balance ===
  Available:  1,500,000 sat
  Required:   200,000 sat
  Burn rate:  12,345 sat/hour
  Runway:     121.5h
  Status:     SUFFICIENT

=== Changes ===
CREATE:
  price:       46001 sat/PH/Day
  speed_limit: 1.0 PH/s
  amount:      100000 sat
  upstream:    stratum+tcp://203.0.113.10:23334 / rig.worker

=== Expected Final State ===
BID  price=46001 sat/PH/Day  limit=1.0 PH/s  amount=100000 sat  (NEW)

# Without --dry-run, the plan is executed.
$ uv run hashbidder set-bids --bid-config bids.toml
...
```

If the **account balance** check fails (insufficient sats to fund planned creates), `set-bids` prints the plan, aborts execution, and exits with status **1**. The same exit code applies when target-hashrate mode cannot proceed after that check.

## Development

```sh
git clone https://github.com/NoFlames-bit/hashbidder.git
cd hashbidder
uv sync --all-groups   # optional: dev dependencies for tests and lint
make check             # format, lint, typecheck, import contracts, tests
make test              # pytest only
```
