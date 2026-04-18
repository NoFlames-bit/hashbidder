package cfg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"
)

func writeCfg(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadConfig_ValidExplicit(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, "c.toml", `
default_amount_sat = 100000

[upstream]
url = "stratum+tcp://pool.example.com:3333"
identity = "worker1"

[[bids]]
price_sat_per_ph_day = 500
speed_limit_ph_s = 5.0

[[bids]]
price_sat_per_ph_day = 300
speed_limit_ph_s = 10.0
`)
	any, err := LoadConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	cfg := any.(domain.SetBidsConfig)
	if cfg.DefaultAmount != 100000 {
		t.Fatal(cfg.DefaultAmount)
	}
	if cfg.Upstream.Identity != "worker1" {
		t.Fatal("upstream")
	}
	if len(cfg.Bids) != 2 {
		t.Fatalf("bids=%d", len(cfg.Bids))
	}
	if cfg.Bids[0].Price.To(domain.PH, domain.Day).Sats != 500 {
		t.Fatal("price0")
	}
}

func TestLoadConfig_BidLevelIdentity(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, "c.toml", `
default_amount_sat = 100000

[upstream]
url = "stratum+tcp://pool.example.com:3333"
identity = "default-worker"

[[bids]]
price_sat_per_ph_day = 500
speed_limit_ph_s = 5.0

[[bids]]
price_sat_per_ph_day = 300
speed_limit_ph_s = 10.0
identity = "rig.other"
`)
	any, err := LoadConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	cfg := any.(domain.SetBidsConfig)
	if cfg.Bids[0].Identity != "" {
		t.Fatalf("bid0 identity=%q", cfg.Bids[0].Identity)
	}
	if cfg.Bids[1].Identity != "rig.other" {
		t.Fatalf("bid1 identity=%q", cfg.Bids[1].Identity)
	}
}

func TestLoadConfig_NormalizesBidsArrayOfTablesHeaderCasing(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, "c.toml", `
default_amount_sat = 44000
[upstream]
url = "stratum+tcp://pool.example.com:3333"

[[bids]]
identity = "worker_a"
price_sat_per_ph_day = 100
speed_limit_ph_s = 1.0

[[Bids]]
identity = "worker_b"
price_sat_per_ph_day = 200
speed_limit_ph_s = 1.0
`)
	any, err := LoadConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	cfg := any.(domain.SetBidsConfig)
	if len(cfg.Bids) != 2 {
		t.Fatalf("expected 2 bid rows after header normalization, got %d", len(cfg.Bids))
	}
	if cfg.Bids[0].Identity != "worker_a" || cfg.Bids[1].Identity != "worker_b" {
		t.Fatalf("identities %q %q", cfg.Bids[0].Identity, cfg.Bids[1].Identity)
	}
}

func TestLoadConfig_URLOnlyUpstream_PerBidIdentities(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, "c.toml", `
default_amount_sat = 44000

[upstream]
url = "stratum+tcp://pool.example.com:3333"

[[bids]]
identity = "bc1qexample.worker_a"
price_sat_per_ph_day = 47000
speed_limit_ph_s = 1.0

[[bids]]
identity = "bc1qexample.worker_b"
price_sat_per_ph_day = 46000
speed_limit_ph_s = 1.0
`)
	any, err := LoadConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	cfg := any.(domain.SetBidsConfig)
	if cfg.Upstream.Identity != "" {
		t.Fatalf("expected empty [upstream].identity, got %q", cfg.Upstream.Identity)
	}
	if len(cfg.Bids) != 2 {
		t.Fatalf("bids=%d", len(cfg.Bids))
	}
	if cfg.Bids[0].Identity != "bc1qexample.worker_a" || cfg.Bids[1].Identity != "bc1qexample.worker_b" {
		t.Fatalf("identities %+v %+v", cfg.Bids[0].Identity, cfg.Bids[1].Identity)
	}
}

func TestLoadConfig_EmptyBids(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, "c.toml", `
default_amount_sat = 50000

[upstream]
url = "stratum+tcp://pool.example.com:3333"
identity = "worker1"
`)
	any, err := LoadConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	cfg := any.(domain.SetBidsConfig)
	if len(cfg.Bids) != 0 {
		t.Fatal("expected empty bids")
	}
}

func TestLoadConfig_EmptyBids_URLOnlyNoUpstreamIdentity(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, "c.toml", `
default_amount_sat = 50000

[upstream]
url = "stratum+tcp://pool.example.com:3333"
`)
	any, err := LoadConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	cfg := any.(domain.SetBidsConfig)
	if cfg.Upstream.Identity != "" {
		t.Fatalf("upstream identity=%q", cfg.Upstream.Identity)
	}
	if len(cfg.Bids) != 0 {
		t.Fatalf("expected no bids, got %d", len(cfg.Bids))
	}
}

func TestLoadConfig_Errors(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name string
		toml string
		want string
	}{
		{"missing_default", `
[upstream]
url = "stratum+tcp://pool.example.com:3333"
identity = "worker1"
`, "default_amount_sat"},
		{"missing_upstream", `
default_amount_sat = 100000
`, "url"},
		{"missing_url", `
default_amount_sat = 100000
[upstream]
identity = "worker1"
`, "url"},
		{"bid_missing_row_identity_when_upstream_url_only", `
default_amount_sat = 100000
[upstream]
url = "stratum+tcp://pool.example.com:3333"
[[bids]]
price_sat_per_ph_day = 500
speed_limit_ph_s = 5.0
`, "identity"},
		{"missing_price", `
default_amount_sat = 100000
[upstream]
url = "stratum+tcp://pool.example.com:3333"
identity = "worker1"
[[bids]]
speed_limit_ph_s = 5.0
`, "price_sat_per_ph_day"},
		{"missing_speed", `
default_amount_sat = 100000
[upstream]
url = "stratum+tcp://pool.example.com:3333"
identity = "worker1"
[[bids]]
price_sat_per_ph_day = 500
`, "speed_limit_ph_s"},
		{"invalid_toml", `this is not valid toml [[[`, "invalid TOML"},
		{"bad_default_type", `
default_amount_sat = "not a number"
[upstream]
url = "stratum+tcp://pool.example.com:3333"
identity = "worker1"
`, "default_amount_sat must be an integer"},
		{"bad_price_type", `
default_amount_sat = 100000
[upstream]
url = "stratum+tcp://pool.example.com:3333"
identity = "worker1"
[[bids]]
price_sat_per_ph_day = "expensive"
speed_limit_ph_s = 5.0
`, "must be an integer"},
		{"zero_speed", `
default_amount_sat = 100000
[upstream]
url = "stratum+tcp://pool.example.com:3333"
identity = "worker1"
[[bids]]
price_sat_per_ph_day = 500
speed_limit_ph_s = 0
`, "positive"},
		{"negative_speed", `
default_amount_sat = 100000
[upstream]
url = "stratum+tcp://pool.example.com:3333"
identity = "worker1"
[[bids]]
price_sat_per_ph_day = 500
speed_limit_ph_s = -1.0
`, "positive"},
		{"bad_upstream_scheme", `
default_amount_sat = 100000
[upstream]
url = "http://pool.example.com:3333"
identity = "worker1"
`, "invalid upstream URL"},
		{"invalid_mode", `
mode = "nonsense"
default_amount_sat = 100000
[upstream]
url = "stratum+tcp://pool.example.com:3333"
identity = "worker1"
`, "invalid mode"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := writeCfg(t, dir, tc.name+".toml", tc.toml)
			_, err := LoadConfig(p)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v want substring %q", err, tc.want)
			}
		})
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig(filepath.Join(t.TempDir(), "nope.toml"))
	if err == nil || !os.IsNotExist(err) {
		t.Fatalf("err=%v", err)
	}
}

func TestLoadConfig_TargetMode(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, "t.toml", `
mode = "target-hashrate"
default_amount_sat = 100000
target_hashrate_ph_s = 10.0
max_bids_count = 3

[upstream]
url = "stratum+tcp://pool.example.com:3333"
identity = "worker1"
`)
	any, err := LoadConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	cfg := any.(TargetHashrateConfig)
	if cfg.MaxBidsCount != 3 || cfg.DefaultAmount != 100000 {
		t.Fatalf("bad cfg %+v", cfg)
	}
}

func TestLoadConfig_TargetRejectsBids(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, "t.toml", `
mode = "target-hashrate"
default_amount_sat = 100000
target_hashrate_ph_s = 10.0
max_bids_count = 3

[upstream]
url = "stratum+tcp://pool.example.com:3333"
identity = "worker1"

[[bids]]
price_sat_per_ph_day = 500
speed_limit_ph_s = 5.0
`)
	_, err := LoadConfig(p)
	if err == nil || !strings.Contains(err.Error(), "target-hashrate") {
		t.Fatalf("err=%v", err)
	}
}

func TestLoadConfig_TargetRequiresUpstreamIdentity(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, "t.toml", `
mode = "target-hashrate"
default_amount_sat = 100000
target_hashrate_ph_s = 10.0
max_bids_count = 3

[upstream]
url = "stratum+tcp://pool.example.com:3333"
`)
	_, err := LoadConfig(p)
	if err == nil || !strings.Contains(err.Error(), "target-hashrate") {
		t.Fatalf("err=%v", err)
	}
}
