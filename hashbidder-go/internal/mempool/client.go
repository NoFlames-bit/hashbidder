package mempool

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"

	"github.com/shopspring/decimal"
)

const DefaultMempoolURL = "https://mempool.bitcoinbarcelona.xyz"

type MempoolError struct {
	StatusCode int
	Message    string
}

func (e *MempoolError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

type ChainStats struct {
	TipHeight  domain.BlockHeight
	Difficulty decimal.Decimal
	TotalFee   domain.Sats
}

type Source interface {
	GetChainStats(blockCount int) (ChainStats, error)
}

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

func (c *Client) raise(resp *http.Response, body []byte) error {
	msg := string(body)
	if msg == "" {
		msg = resp.Status
	}
	return &MempoolError{StatusCode: resp.StatusCode, Message: msg}
}

func (c *Client) GetChainStats(blockCount int) (ChainStats, error) {
	statsURL := fmt.Sprintf("%s/api/v1/mining/reward-stats/%d", c.BaseURL, blockCount)
	resp, err := c.HTTP.Get(statsURL)
	if err != nil {
		return ChainStats{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ChainStats{}, c.raise(resp, b)
	}
	var stats struct {
		EndBlock any `json:"endBlock"`
		TotalFee any `json:"totalFee"`
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.UseNumber()
	if err := dec.Decode(&stats); err != nil {
		return ChainStats{}, err
	}
	tipStr := fmt.Sprint(stats.EndBlock)
	tipVal, err := parseIntString(tipStr)
	if err != nil {
		return ChainStats{}, err
	}
	tip, err := domain.NewBlockHeight(tipVal)
	if err != nil {
		return ChainStats{}, err
	}
	feeStr := fmt.Sprint(stats.TotalFee)
	feeVal, err := parseIntString(feeStr)
	if err != nil {
		return ChainStats{}, err
	}

	blockURL := fmt.Sprintf("%s/api/v1/blocks/%d", c.BaseURL, tip.Value)
	resp2, err := c.HTTP.Get(blockURL)
	if err != nil {
		return ChainStats{}, err
	}
	defer func() { _ = resp2.Body.Close() }()
	b2, _ := io.ReadAll(resp2.Body)
	if resp2.StatusCode < 200 || resp2.StatusCode >= 300 {
		return ChainStats{}, c.raise(resp2, b2)
	}
	var blocks []map[string]any
	dec2 := json.NewDecoder(strings.NewReader(string(b2)))
	dec2.UseNumber()
	if err := dec2.Decode(&blocks); err != nil {
		return ChainStats{}, err
	}
	if len(blocks) == 0 {
		return ChainStats{}, fmt.Errorf("empty blocks response")
	}
	diffRaw := blocks[0]["difficulty"]
	diffStr := fmt.Sprint(diffRaw)
	diff, err := decimal.NewFromString(diffStr)
	if err != nil {
		return ChainStats{}, err
	}
	return ChainStats{
		TipHeight:  tip,
		Difficulty: diff,
		TotalFee:   domain.Sats(feeVal),
	}, nil
}

func parseIntString(s string) (int, error) {
	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil {
		return 0, err
	}
	return int(n), nil
}
