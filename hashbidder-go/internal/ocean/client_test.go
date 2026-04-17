package ocean

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"

	"github.com/shopspring/decimal"
)

const validHTML = `
<tr class="table-row">
  <td class="table-cell">24 hrs</td>
  <td class="table-cell">1885.8 Th/s</td>
  <td class="table-cell">123 shares</td>
</tr>
<tr class="table-row">
  <td class="table-cell">3 hrs</td>
  <td class="table-cell">1850.0 Th/s</td>
  <td class="table-cell">45 shares</td>
</tr>
<tr class="table-row">
  <td class="table-cell">10 min</td>
  <td class="table-cell">3.22 Th/s</td>
  <td class="table-cell">5 shares</td>
</tr>
<tr class="table-row">
  <td class="table-cell">5 min</td>
  <td class="table-cell">3.02 Th/s</td>
  <td class="table-cell">3 shares</td>
</tr>
<tr class="table-row">
  <td class="table-cell">60 sec</td>
  <td class="table-cell">3.00 Th/s</td>
  <td class="table-cell">1 shares</td>
</tr>
`

func TestOceanClient_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "user=") {
			t.Fatal("missing user param")
		}
		_, _ = w.Write([]byte(validHTML))
	}))
	defer srv.Close()
	addr, err := domain.ParseBtcAddress("bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4")
	if err != nil {
		t.Fatal(err)
	}
	c := NewClient(srv.URL, srv.Client())
	stats, err := c.GetAccountStats(addr)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats.Windows) != 5 {
		t.Fatalf("windows=%d", len(stats.Windows))
	}
	if stats.Windows[0].Window != WindowDay {
		t.Fatal(stats.Windows[0].Window)
	}
	if !stats.Windows[0].Hashrate.Value.Equal(decimal.RequireFromString("1885.8")) || stats.Windows[0].Hashrate.HashUnit != domain.TH {
		t.Fatalf("24h=%+v", stats.Windows[0].Hashrate)
	}
	if stats.Windows[4].Window != WindowSixtySeconds || !stats.Windows[4].Hashrate.Value.Equal(decimal.RequireFromString("3.00")) {
		t.Fatalf("60s=%+v", stats.Windows[4])
	}
}

func TestOceanClient_WrongRows(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `<tr class="table-row"><td class="table-cell">24 hrs</td><td class="table-cell">0.00 Th/s</td><td class="table-cell">0</td></tr>`
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()
	addr, _ := domain.ParseBtcAddress("bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4")
	c := NewClient(srv.URL, srv.Client())
	_, err := c.GetAccountStats(addr)
	if err == nil || !strings.Contains(err.Error(), "expected 5 rows") {
		t.Fatalf("err=%v", err)
	}
}
