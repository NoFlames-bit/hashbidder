package braiins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"

	"github.com/shopspring/decimal"
)

const APIBase = "https://hashpower.braiins.com/v1"

type ClOrderID string

type OrderBook struct {
	Bids []BidItem
	Asks []AskItem
}

type BidItem struct {
	Price        domain.HashratePrice
	AmountSat    domain.Sats
	HrMatchedPH  domain.Hashrate
	SpeedLimitPH domain.Hashrate
}

type AskItem struct {
	Price         domain.HashratePrice
	HrMatchedPH   domain.Hashrate
	HrAvailablePH domain.Hashrate
}

type MarketSettings struct {
	MinBidPriceDecreasePeriod      time.Duration
	MinBidSpeedLimitDecreasePeriod time.Duration
	PriceTick                      domain.PriceTick
}

type AccountBalance struct {
	AvailableSat domain.Sats
	BlockedSat   domain.Sats
	TotalSat     domain.Sats
}

type CreateBidResult struct {
	ID domain.BidID
}

type HashpowerClient interface {
	GetOrderbook() (OrderBook, error)
	GetCurrentBids() ([]domain.UserBid, error)
	CreateBid(up domain.Upstream, amount domain.Sats, price domain.HashratePrice, speed domain.Hashrate, cl ClOrderID) (CreateBidResult, error)
	EditBid(id domain.BidID, newPrice domain.HashratePrice, newSpeed domain.Hashrate) error
	CancelBid(id domain.BidID) error
	GetMarketSettings() (MarketSettings, error)
	GetAccountBalance() (AccountBalance, error)
}

type Client struct {
	BaseURL string
	APIKey  *string
	HTTP    *http.Client
}

func NewClient(base string, apiKey *string, hc *http.Client) *Client {
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{BaseURL: strings.TrimRight(base, "/"), APIKey: apiKey, HTTP: hc}
}

type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

func (e *APIError) IsTransient() bool {
	return e.StatusCode == 429 || e.StatusCode >= 500
}

func (c *Client) authHeaders() (http.Header, error) {
	if c.APIKey == nil || *c.APIKey == "" {
		return nil, fmt.Errorf("API key required for authenticated endpoints")
	}
	h := http.Header{}
	h.Set("apikey", *c.APIKey)
	return h, nil
}

func (c *Client) raiseAPIError(resp *http.Response, body []byte) error {
	msg := resp.Header.Get("grpc-message")
	if msg != "" {
		un, err := url.PathUnescape(msg)
		if err == nil {
			msg = un
		}
	} else {
		var data map[string]any
		if json.Unmarshal(body, &data) == nil {
			if m, ok := data["message"].(string); ok {
				msg = m
			}
		}
		if msg == "" {
			msg = string(body)
			if msg == "" {
				msg = resp.Status
			}
		}
	}
	return &APIError{StatusCode: resp.StatusCode, Message: msg}
}

func decFromAny(v any) (decimal.Decimal, error) {
	switch x := v.(type) {
	case json.Number:
		return decimal.NewFromString(x.String())
	case float64:
		return decimal.NewFromFloat(x), nil
	case string:
		return decimal.NewFromString(x)
	default:
		return decimal.Zero, fmt.Errorf("unsupported number type %T", v)
	}
}

func (c *Client) GetOrderbook() (OrderBook, error) {
	u := c.BaseURL + "/spot/orderbook"
	resp, err := c.HTTP.Get(u)
	if err != nil {
		return OrderBook{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return OrderBook{}, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}
	var data struct {
		Bids []map[string]any `json:"bids"`
		Asks []map[string]any `json:"asks"`
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&data); err != nil {
		return OrderBook{}, err
	}
	parseBid := func(item map[string]any) (BidItem, error) {
		ps, err := decFromAny(item["price_sat"])
		if err != nil {
			return BidItem{}, err
		}
		price, _ := domain.NewHashratePrice(domain.Sats(ps.IntPart()), mustHR(decimal.NewFromInt(1), domain.EH, domain.Day))
		asat, _ := decFromAny(item["amount_sat"])
		hm, _ := decFromAny(item["hr_matched_ph"])
		sl, _ := decFromAny(item["speed_limit_ph"])
		hmHR, _ := domain.NewHashrate(hm, domain.PH, domain.Second)
		slHR, _ := domain.NewHashrate(sl, domain.PH, domain.Second)
		return BidItem{
			Price:        price,
			AmountSat:    domain.Sats(asat.IntPart()),
			HrMatchedPH:  hmHR,
			SpeedLimitPH: slHR,
		}, nil
	}
	parseAsk := func(item map[string]any) (AskItem, error) {
		ps, err := decFromAny(item["price_sat"])
		if err != nil {
			return AskItem{}, err
		}
		price, _ := domain.NewHashratePrice(domain.Sats(ps.IntPart()), mustHR(decimal.NewFromInt(1), domain.EH, domain.Day))
		hm, _ := decFromAny(item["hr_matched_ph"])
		ha, _ := decFromAny(item["hr_available_ph"])
		hmHR, _ := domain.NewHashrate(hm, domain.PH, domain.Second)
		haHR, _ := domain.NewHashrate(ha, domain.PH, domain.Second)
		return AskItem{Price: price, HrMatchedPH: hmHR, HrAvailablePH: haHR}, nil
	}
	bids := make([]BidItem, 0, len(data.Bids))
	for _, it := range data.Bids {
		bi, err := parseBid(it)
		if err != nil {
			return OrderBook{}, err
		}
		bids = append(bids, bi)
	}
	asks := make([]AskItem, 0, len(data.Asks))
	for _, it := range data.Asks {
		ai, err := parseAsk(it)
		if err != nil {
			return OrderBook{}, err
		}
		asks = append(asks, ai)
	}
	return OrderBook{Bids: bids, Asks: asks}, nil
}

func mustHR(v decimal.Decimal, hu domain.HashUnit, tu domain.TimeUnit) domain.Hashrate {
	h, err := domain.NewHashrate(v, hu, tu)
	if err != nil {
		panic(err)
	}
	return h
}

func (c *Client) GetCurrentBids() ([]domain.UserBid, error) {
	h, err := c.authHeaders()
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequest(http.MethodGet, c.BaseURL+"/spot/bid/current", nil)
	req.Header = h
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}
	var data struct {
		Items []map[string]any `json:"items"`
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&data); err != nil {
		return nil, err
	}
	out := make([]domain.UserBid, 0, len(data.Items))
	for _, item := range data.Items {
		ub, err := parseUserBid(item)
		if err != nil {
			return nil, err
		}
		out = append(out, ub)
	}
	return out, nil
}

func bidMapString(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case json.Number:
		return strings.TrimSpace(x.String())
	default:
		return strings.TrimSpace(fmt.Sprint(x))
	}
}

func parseUserBid(item map[string]any) (domain.UserBid, error) {
	bid, ok := item["bid"].(map[string]any)
	if !ok || bid == nil {
		return domain.UserBid{}, fmt.Errorf("missing bid object")
	}
	var state map[string]any
	if s, ok := item["state_estimate"].(map[string]any); ok {
		state = s
	}
	priceSat, _ := decFromAny(bid["price_sat"])
	price, _ := domain.NewHashratePrice(domain.Sats(priceSat.IntPart()), mustHR(decimal.NewFromInt(1), domain.EH, domain.Day))
	sl, _ := decFromAny(bid["speed_limit_ph"])
	speed, _ := domain.NewHashrate(sl, domain.PH, domain.Second)
	amt, _ := decFromAny(bid["amount_sat"])
	st := domain.BidStatus(bidMapString(bid, "status"))
	lu, err := time.Parse(time.RFC3339Nano, bid["last_updated"].(string))
	if err != nil {
		lu, _ = time.Parse(time.RFC3339, bid["last_updated"].(string))
	}
	var prog *domain.Progress
	var rem *domain.Sats
	if state != nil {
		if pctRaw, ok := state["progress_pct"]; ok {
			pct, err := decFromAny(pctRaw)
			if err == nil {
				p, err := domain.ProgressFromPercentage(pct)
				if err == nil {
					prog = &p
				}
			}
		}
		if ar, ok := state["amount_remaining_sat"]; ok {
			v, err := decFromAny(ar)
			if err == nil {
				rs := domain.Sats(v.IntPart())
				rem = &rs
			}
		}
	}
	var up *domain.Upstream
	if du, ok := bid["dest_upstream"].(map[string]any); ok && du != nil {
		rawURL := strings.TrimSpace(fmt.Sprint(du["url"]))
		su, err := domain.ParseStratumURL(rawURL)
		if err == nil {
			id := strings.TrimSpace(fmt.Sprint(du["identity"]))
			up = &domain.Upstream{URL: su, Identity: id}
		}
	}
	return domain.UserBid{
		ID:                 domain.BidID(bidMapString(bid, "id")),
		Price:              price,
		SpeedLimitPH:       speed,
		AmountSat:          domain.Sats(amt.IntPart()),
		Status:             st,
		Progress:           prog,
		AmountRemainingSat: rem,
		LastUpdated:        lu,
		Upstream:           up,
	}, nil
}

func (c *Client) priceToAPI(price domain.HashratePrice) int64 {
	return int64(price.To(domain.EH, domain.Day).Sats)
}

func (c *Client) speedToAPI(speed domain.Hashrate) float64 {
	v := speed.To(domain.PH, domain.Second).Value
	f, _ := v.Float64()
	return f
}

func (c *Client) CreateBid(up domain.Upstream, amount domain.Sats, price domain.HashratePrice, speed domain.Hashrate, cl ClOrderID) (CreateBidResult, error) {
	h, err := c.authHeaders()
	if err != nil {
		return CreateBidResult{}, err
	}
	body := map[string]any{
		"dest_upstream": map[string]string{
			"url":      up.URL.String(),
			"identity": up.Identity,
		},
		"amount_sat":     int64(amount),
		"price_sat":      c.priceToAPI(price),
		"speed_limit_ph": c.speedToAPI(speed),
		"cl_order_id":    string(cl),
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, c.BaseURL+"/spot/bid", bytes.NewReader(b))
	req.Header = h
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return CreateBidResult{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return CreateBidResult{}, c.raiseAPIError(resp, rb)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rb, &out); err != nil {
		return CreateBidResult{}, err
	}
	return CreateBidResult{ID: domain.BidID(out.ID)}, nil
}

func (c *Client) EditBid(id domain.BidID, newPrice domain.HashratePrice, newSpeed domain.Hashrate) error {
	h, err := c.authHeaders()
	if err != nil {
		return err
	}
	body := map[string]any{
		"bid_id":             string(id),
		"new_price_sat":      c.priceToAPI(newPrice),
		"new_speed_limit_ph": map[string]float64{"value": c.speedToAPI(newSpeed)},
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPut, c.BaseURL+"/spot/bid", bytes.NewReader(b))
	req.Header = h
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.raiseAPIError(resp, rb)
	}
	return nil
}

func (c *Client) CancelBid(id domain.BidID) error {
	h, err := c.authHeaders()
	if err != nil {
		return err
	}
	body := map[string]string{"order_id": string(id)}
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, c.BaseURL+"/spot/bid", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header = h
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(b))
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.raiseAPIError(resp, rb)
	}
	return nil
}

func (c *Client) GetMarketSettings() (MarketSettings, error) {
	h, err := c.authHeaders()
	if err != nil {
		return MarketSettings{}, err
	}
	req, _ := http.NewRequest(http.MethodGet, c.BaseURL+"/spot/settings", nil)
	req.Header = h
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return MarketSettings{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return MarketSettings{}, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}
	var data struct {
		MinBidPriceDecreasePeriodS      int `json:"min_bid_price_decrease_period_s"`
		MinBidSpeedLimitDecreasePeriodS int `json:"min_bid_speed_limit_decrease_period_s"`
		TickSizeSat                     int `json:"tick_size_sat"`
	}
	if err := json.Unmarshal(b, &data); err != nil {
		return MarketSettings{}, err
	}
	tick, err := domain.NewPriceTick(domain.Sats(data.TickSizeSat))
	if err != nil {
		return MarketSettings{}, err
	}
	return MarketSettings{
		MinBidPriceDecreasePeriod:      time.Duration(data.MinBidPriceDecreasePeriodS) * time.Second,
		MinBidSpeedLimitDecreasePeriod: time.Duration(data.MinBidSpeedLimitDecreasePeriodS) * time.Second,
		PriceTick:                      tick,
	}, nil
}

func (c *Client) GetAccountBalance() (AccountBalance, error) {
	h, err := c.authHeaders()
	if err != nil {
		return AccountBalance{}, err
	}
	req, _ := http.NewRequest(http.MethodGet, c.BaseURL+"/account/balance", nil)
	req.Header = h
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return AccountBalance{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return AccountBalance{}, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}
	var data struct {
		Accounts []struct {
			Available int64 `json:"available_balance_sat"`
			Blocked   int64 `json:"blocked_balance_sat"`
			Total     int64 `json:"total_balance_sat"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(b, &data); err != nil {
		return AccountBalance{}, err
	}
	if len(data.Accounts) != 1 {
		return AccountBalance{}, fmt.Errorf("expected exactly one account in balance response, got %d", len(data.Accounts))
	}
	a := data.Accounts[0]
	return AccountBalance{
		AvailableSat: domain.Sats(a.Available),
		BlockedSat:   domain.Sats(a.Blocked),
		TotalSat:     domain.Sats(a.Total),
	}, nil
}
