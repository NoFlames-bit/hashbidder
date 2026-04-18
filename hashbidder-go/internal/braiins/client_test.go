package braiins

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"

	"github.com/shopspring/decimal"
)

var testUpstream = func() domain.Upstream {
	u, _ := domain.ParseStratumURL("stratum+tcp://pool.example.com:3333")
	return domain.Upstream{URL: u, Identity: "worker1"}
}()

func TestCreateBid_Body(t *testing.T) {
	var captured *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		r2 := *r
		r2.Body = io.NopCloser(bytes.NewReader(b))
		captured = &r2
		_, _ = w.Write([]byte(`{"id":"B999"}`))
	}))
	defer srv.Close()
	key := "test-api-key"
	c := NewClient(srv.URL, &key, srv.Client())
	per, _ := domain.NewHashrate(decimal.NewFromInt(1), domain.PH, domain.Day)
	pr, _ := domain.NewHashratePrice(domain.Sats(500), per)
	sp, _ := domain.NewHashrate(decimal.RequireFromString("5.0"), domain.PH, domain.Second)
	res, err := c.CreateBid(testUpstream, 100_000, pr, sp, "order-123")
	if err != nil || res.ID != "B999" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	body, _ := io.ReadAll(captured.Body)
	if !bytes.Contains(body, []byte(`"amount_sat":100000`)) {
		t.Fatalf("body=%s", string(body))
	}
	if !bytes.Contains(body, []byte(`"price_sat":500000`)) {
		t.Fatalf("body=%s", string(body))
	}
	if !bytes.Contains(body, []byte(`"speed_limit_ph":5`)) {
		t.Fatalf("body=%s", string(body))
	}
	if captured.Header.Get("apikey") != key {
		t.Fatal("missing apikey")
	}
}

func TestEditBid_Body(t *testing.T) {
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(200)
	}))
	defer srv.Close()
	key := "k"
	c := NewClient(srv.URL, &key, srv.Client())
	per, _ := domain.NewHashrate(decimal.NewFromInt(1), domain.PH, domain.Day)
	pr, _ := domain.NewHashratePrice(domain.Sats(300), per)
	sp, _ := domain.NewHashrate(decimal.RequireFromString("10.0"), domain.PH, domain.Second)
	if err := c.EditBid("B123", pr, sp); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte(`"bid_id":"B123"`)) || !bytes.Contains(body, []byte(`"new_price_sat":300000`)) {
		t.Fatalf("body=%s", string(body))
	}
	if !bytes.Contains(body, []byte(`"new_speed_limit_ph":{"value":10`)) {
		t.Fatalf("body=%s", string(body))
	}
}

func TestCancelBid_Body(t *testing.T) {
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(200)
	}))
	defer srv.Close()
	key := "k"
	c := NewClient(srv.URL, &key, srv.Client())
	if err := c.CancelBid("B456"); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte(`"order_id":"B456"`)) {
		t.Fatalf("body=%s", string(body))
	}
}

func TestGetCurrentBids_JSONNumberIDStratumSSL(t *testing.T) {
	const payload = `{"items":[{"bid":{"id":42,"price_sat":"500000","speed_limit_ph":"5","amount_sat":"100000","status":"BID_STATUS_ACTIVE","last_updated":"1970-01-01T00:00:00Z","dest_upstream":{"url":"stratum+ssl://pool.example.com:3333","identity":"  w1  "}}}]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/spot/bid/current" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	key := "k"
	c := NewClient(srv.URL, &key, srv.Client())
	bids, err := c.GetCurrentBids()
	if err != nil {
		t.Fatal(err)
	}
	if len(bids) != 1 {
		t.Fatalf("len=%d bids=%+v", len(bids), bids)
	}
	if bids[0].ID != "42" {
		t.Fatalf("id=%q", bids[0].ID)
	}
	if bids[0].Upstream == nil {
		t.Fatal("expected upstream")
	}
	if bids[0].Upstream.Identity != "w1" {
		t.Fatalf("identity=%q", bids[0].Upstream.Identity)
	}
	if bids[0].Upstream.URL.Scheme() != "stratum+ssl" {
		t.Fatalf("scheme=%q", bids[0].Upstream.URL.Scheme())
	}
}

func TestGetBidHistory_JSON(t *testing.T) {
	const payload = `{"history":[{"timestamp":"2026-04-17T08:00:00Z","price_sat":"500000","speed_limit_ph":"10"},{"timestamp":"2026-04-17T07:00:00Z","price_sat":"600000","speed_limit_ph":"5"}]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/spot/bid/detail/B42" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	key := "k"
	c := NewClient(srv.URL, &key, srv.Client())
	h, err := c.GetBidHistory("B42")
	if err != nil {
		t.Fatal(err)
	}
	entries := h.Entries()
	if len(entries) != 2 {
		t.Fatalf("len=%d", len(entries))
	}
	if entries[0].Price.Sats != 500_000 || !entries[0].SpeedLimitPH.Value.Equal(decimal.RequireFromString("10")) {
		t.Fatalf("newest entry %+v", entries[0])
	}
	if entries[1].Price.Sats != 600_000 {
		t.Fatalf("older entry %+v", entries[1])
	}
	if got := h.LastPriceDecreaseAt(); got == nil || !got.Equal(entries[0].Timestamp) {
		t.Fatalf("last price decrease %v", got)
	}
	if h.LastSpeedDecreaseAt() != nil {
		t.Fatalf("expected no speed decrease, got %v", h.LastSpeedDecreaseAt())
	}
}

// Bid IDs with reserved path characters must be escaped so one segment does not
// become multiple path segments on the server.
func TestGetBidHistory_pathEscapesSpecialBidID(t *testing.T) {
	const payload = `{"history":[{"timestamp":"2026-04-17T08:00:00Z","price_sat":"100","speed_limit_ph":"1"}]}`
	bidID := domain.BidID("B/extra") // slash must not appear as an extra path segment

	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.URL.Path != "/spot/bid/detail/B/extra" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	key := "k"
	c := NewClient(srv.URL, &key, srv.Client())
	h, err := c.GetBidHistory(bidID)
	if err != nil {
		t.Fatalf("GetBidHistory: %v (server path was %q)", err, gotPath)
	}
	if gotPath != "/spot/bid/detail/B/extra" {
		t.Fatalf("server path %q: expected single logical segment after decode", gotPath)
	}
	if len(h.Entries()) != 1 {
		t.Fatalf("entries=%v", h.Entries())
	}
}
