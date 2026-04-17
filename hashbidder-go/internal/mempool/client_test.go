package mempool

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"

	"github.com/shopspring/decimal"
)

func TestClient_GetChainStats_OK(t *testing.T) {
	var reqs int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqs++
		switch r.URL.Path {
		case "/api/v1/mining/reward-stats/2016":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"startBlock": 838000,
				"endBlock":   840000,
				"totalFee":   "50000000000",
			})
		case "/api/v1/blocks/840000":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"difficulty": 83148355189239.77}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, srv.Client())
	stats, err := c.GetChainStats(2016)
	if err != nil {
		t.Fatal(err)
	}
	if stats.TipHeight.Value != 840000 {
		t.Fatalf("tip=%v", stats.TipHeight)
	}
	if !stats.Difficulty.Equal(decimal.RequireFromString("83148355189239.77")) {
		t.Fatalf("diff=%s", stats.Difficulty)
	}
	if stats.TotalFee != domain.Sats(50_000_000_000) {
		t.Fatalf("fee=%d", stats.TotalFee)
	}
	if reqs != 2 {
		t.Fatalf("reqs=%d", reqs)
	}
}

func TestClient_GetChainStats_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())
	_, err := c.GetChainStats(2016)
	if err == nil {
		t.Fatal("expected error")
	}
	var me *MempoolError
	if !errors.As(err, &me) || me.StatusCode != 503 {
		t.Fatalf("err=%v", err)
	}
}
