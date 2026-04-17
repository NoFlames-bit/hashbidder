package testutil

import (
	"fmt"
	"sync"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/braiins"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/testhelpers"

	"github.com/shopspring/decimal"
)

// FakeClient is a stateful in-memory HashpowerClient for tests (see tests/conftest.py).
type FakeClient struct {
	Orderbook      braiins.OrderBook
	Bids           []domain.UserBid
	NextID         int
	Errors         map[string][]*braiins.APIError // key: "method:id"
	MarketSettings braiins.MarketSettings
	AccountBalance braiins.AccountBalance
	Calls          [][]string

	FailFirstCreate bool
	createFailOnce  bool

	mu sync.Mutex
}

func NewFakeClient(opts ...FakeOption) *FakeClient {
	tick, err := domain.NewPriceTick(1000)
	if err != nil {
		panic(err)
	}
	c := &FakeClient{
		Orderbook: braiins.OrderBook{},
		Bids:      nil,
		NextID:    1,
		Errors:    map[string][]*braiins.APIError{},
		MarketSettings: braiins.MarketSettings{
			MinBidPriceDecreasePeriod:      0,
			MinBidSpeedLimitDecreasePeriod: 0,
			PriceTick:                      tick,
		},
		AccountBalance: braiins.AccountBalance{
			AvailableSat: 10_000_000_000,
			BlockedSat:   0,
			TotalSat:     10_000_000_000,
		},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

type FakeOption func(*FakeClient)

func WithCurrentBids(bids ...domain.UserBid) FakeOption {
	return func(c *FakeClient) {
		c.Bids = append([]domain.UserBid(nil), bids...)
	}
}

func WithOrderbook(ob braiins.OrderBook) FakeOption {
	return func(c *FakeClient) { c.Orderbook = ob }
}

func WithErrors(errs map[string][]*braiins.APIError) FakeOption {
	return func(c *FakeClient) { c.Errors = errs }
}

func WithAccountBalance(b braiins.AccountBalance) FakeOption {
	return func(c *FakeClient) { c.AccountBalance = b }
}

func (c *FakeClient) errKey(method, id string) string {
	return method + ":" + id
}

func (c *FakeClient) maybeRaise(method, key string) error {
	k := c.errKey(method, key)
	if errs, ok := c.Errors[k]; ok && len(errs) > 0 {
		e := errs[0]
		c.Errors[k] = errs[1:]
		return e
	}
	return nil
}

func (c *FakeClient) record(parts ...string) {
	c.Calls = append(c.Calls, parts)
}

func (c *FakeClient) GetMarketSettings() (braiins.MarketSettings, error) {
	return c.MarketSettings, nil
}

func (c *FakeClient) GetAccountBalance() (braiins.AccountBalance, error) {
	return c.AccountBalance, nil
}

func (c *FakeClient) GetOrderbook() (braiins.OrderBook, error) {
	return c.Orderbook, nil
}

func (c *FakeClient) GetCurrentBids() ([]domain.UserBid, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]domain.UserBid, len(c.Bids))
	copy(out, c.Bids)
	return out, nil
}

func (c *FakeClient) CreateBid(up domain.Upstream, amount domain.Sats, price domain.HashratePrice, speed domain.Hashrate, cl braiins.ClOrderID) (braiins.CreateBidResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.record("create_bid", string(cl))
	if err := c.maybeRaise("create_bid", string(cl)); err != nil {
		return braiins.CreateBidResult{}, err
	}
	if c.FailFirstCreate && !c.createFailOnce {
		c.createFailOnce = true
		return braiins.CreateBidResult{}, &braiins.APIError{StatusCode: 400, Message: "insufficient balance"}
	}
	id := domain.BidID(fmt.Sprintf("B%09d", c.NextID))
	c.NextID++
	prog, _ := domain.ProgressFromPercentage(decimal.Zero)
	c.Bids = append(c.Bids, domain.UserBid{
		ID:                 id,
		Price:              price,
		SpeedLimitPH:       speed,
		AmountSat:          amount,
		Status:             domain.BidStatusCreated,
		Progress:           &prog,
		AmountRemainingSat: ptrSatsAmount(int(amount)),
		LastUpdated:        testhelpers.DefaultLastUpdated,
		Upstream:           &up,
	})
	return braiins.CreateBidResult{ID: id}, nil
}

func (c *FakeClient) EditBid(bidID domain.BidID, newPrice domain.HashratePrice, newSpeed domain.Hashrate) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.record("edit_bid", string(bidID))
	if err := c.maybeRaise("edit_bid", string(bidID)); err != nil {
		return err
	}
	for i := range c.Bids {
		if c.Bids[i].ID == bidID {
			b := c.Bids[i]
			c.Bids[i] = domain.UserBid{
				ID:                 b.ID,
				Price:              newPrice,
				SpeedLimitPH:       newSpeed,
				AmountSat:          b.AmountSat,
				Status:             b.Status,
				Progress:           b.Progress,
				AmountRemainingSat: b.AmountRemainingSat,
				LastUpdated:        b.LastUpdated,
				Upstream:           b.Upstream,
			}
			return nil
		}
	}
	return &braiins.APIError{StatusCode: 404, Message: fmt.Sprintf("Bid %s not found", bidID)}
}

func ptrSatsAmount(v int) *domain.Sats {
	s := domain.Sats(v)
	return &s
}

func (c *FakeClient) CancelBid(orderID domain.BidID) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.record("cancel_bid", string(orderID))
	if err := c.maybeRaise("cancel_bid", string(orderID)); err != nil {
		return err
	}
	for i := range c.Bids {
		if c.Bids[i].ID == orderID {
			c.Bids = append(c.Bids[:i], c.Bids[i+1:]...)
			return nil
		}
	}
	return &braiins.APIError{StatusCode: 404, Message: fmt.Sprintf("Bid %s not found", orderID)}
}
