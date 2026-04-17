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
