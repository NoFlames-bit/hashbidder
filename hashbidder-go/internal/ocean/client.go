package ocean

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"

	"github.com/shopspring/decimal"
)

const DefaultOceanURL = "https://ocean.xyz"

type OceanTimeWindow string

const (
	WindowDay          OceanTimeWindow = "24 hrs"
	WindowThreeHours   OceanTimeWindow = "3 hrs"
	WindowTenMinutes   OceanTimeWindow = "10 min"
	WindowFiveMinutes  OceanTimeWindow = "5 min"
	WindowSixtySeconds OceanTimeWindow = "60 sec"
)

type OceanError struct {
	StatusCode int
	Message    string
}

func (e *OceanError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

type HashrateWindow struct {
	Window   OceanTimeWindow
	Hashrate domain.Hashrate
}

type AccountStats struct {
	Windows []HashrateWindow
}

type Source interface {
	GetAccountStats(address domain.BtcAddress) (AccountStats, error)
}

var rowRe = regexp.MustCompile(`(?s)<tr\s+class="table-row">(.*?)</tr>`)
var cellRe = regexp.MustCompile(`(?s)<td\s+class="table-cell"\s*>(.*?)</td>`)

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func NewClient(base string, hc *http.Client) *Client {
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{BaseURL: strings.TrimRight(base, "/"), HTTP: hc}
}

func parseHashrate(text string) (domain.Hashrate, error) {
	parts := strings.Fields(strings.TrimSpace(text))
	if len(parts) != 2 {
		return domain.Hashrate{}, &OceanError{200, fmt.Sprintf("unexpected hashrate format: %q", text)}
	}
	value, err := decimal.NewFromString(parts[0])
	if err != nil {
		return domain.Hashrate{}, &OceanError{200, fmt.Sprintf("invalid hashrate value: %q", parts[0])}
	}
	u, err := domain.HashUnitFromRateStr(parts[1])
	if err != nil {
		return domain.Hashrate{}, &OceanError{200, err.Error()}
	}
	return domain.NewHashrate(value, u, domain.Second)
}

func parseHTML(html string) (AccountStats, error) {
	rows := rowRe.FindAllStringSubmatch(html, -1)
	if len(rows) != 5 {
		return AccountStats{}, &OceanError{200, fmt.Sprintf("expected 5 rows, got %d; response schema may have changed", len(rows))}
	}
	expected := []OceanTimeWindow{WindowDay, WindowThreeHours, WindowTenMinutes, WindowFiveMinutes, WindowSixtySeconds}
	windows := make([]HashrateWindow, 0, 5)
	for i, m := range rows {
		rowHTML := m[1]
		cells := cellRe.FindAllStringSubmatch(rowHTML, -1)
		if len(cells) != 3 {
			return AccountStats{}, &OceanError{200, fmt.Sprintf("row %d: expected 3 cells, got %d", i, len(cells))}
		}
		label := strings.TrimSpace(cells[0][1])
		exp := string(expected[i])
		if label != exp {
			return AccountStats{}, &OceanError{200, fmt.Sprintf("row %d: expected period %q, got %q", i, exp, label)}
		}
		hr, err := parseHashrate(cells[1][1])
		if err != nil {
			return AccountStats{}, err
		}
		windows = append(windows, HashrateWindow{Window: expected[i], Hashrate: hr})
	}
	return AccountStats{Windows: windows}, nil
}

func (c *Client) GetAccountStats(address domain.BtcAddress) (AccountStats, error) {
	u := c.BaseURL + "/template/workers/hashrates/rows"
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return AccountStats{}, err
	}
	q := req.URL.Query()
	q.Set("user", address.Value())
	req.URL.RawQuery = q.Encode()
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return AccountStats{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := string(b)
		if msg == "" {
			msg = resp.Status
		}
		return AccountStats{}, &OceanError{resp.StatusCode, msg}
	}
	return parseHTML(string(b))
}
