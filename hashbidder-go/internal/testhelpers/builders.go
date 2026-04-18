package testhelpers

import (
	"time"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"

	"github.com/shopspring/decimal"
)

var (
	DefaultLastUpdated = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

	UpstreamPool  = mustUpstream("stratum+tcp://pool.example.com:3333", "worker1")
	OtherUpstream = mustUpstream("stratum+tcp://other.pool.com:4444", "worker2")
)

func mustUpstream(urlStr, identity string) domain.Upstream {
	u, err := domain.ParseStratumURL(urlStr)
	if err != nil {
		panic(err)
	}
	return domain.Upstream{URL: u, Identity: identity}
}

func ehDay() domain.Hashrate {
	h, err := domain.NewHashrate(decimal.NewFromInt(1), domain.EH, domain.Day)
	if err != nil {
		panic(err)
	}
	return h
}

func phDay() domain.Hashrate {
	h, err := domain.NewHashrate(decimal.NewFromInt(1), domain.PH, domain.Day)
	if err != nil {
		panic(err)
	}
	return h
}

// MakeUserBid mirrors tests/conftest.make_user_bid: price is sat/PH/Day; stored as sat/EH/Day (*1000).
func MakeUserBid(bidID string, priceSatPerPHDay int, speed string, opts ...UserBidOption) domain.UserBid {
	cfg := userBidCfg{
		status:      domain.BidStatusActive,
		amount:      100_000,
		upstream:    &UpstreamPool,
		lastUpdated: DefaultLastUpdated,
		pricePerPH:  priceSatPerPHDay,
		speedPHS:    speed,
	}
	for _, o := range opts {
		o(&cfg)
	}
	if !cfg.remainingSet {
		cfg.remaining = ptrSats(cfg.amount)
	}
	speedDec, err := decimal.NewFromString(cfg.speedPHS)
	if err != nil {
		panic(err)
	}
	slim, err := domain.NewHashrate(speedDec, domain.PH, domain.Second)
	if err != nil {
		panic(err)
	}
	price, err := domain.NewHashratePrice(domain.Sats(int64(cfg.pricePerPH)*1000), ehDay())
	if err != nil {
		panic(err)
	}
	prog, err := domain.ProgressFromPercentage(decimal.Zero)
	if err != nil {
		panic(err)
	}
	return domain.UserBid{
		ID:                 domain.BidID(bidID),
		Price:              price,
		SpeedLimitPH:       slim,
		AmountSat:          domain.Sats(cfg.amount),
		Status:             cfg.status,
		Progress:           &prog,
		AmountRemainingSat: cfg.remaining,
		LastUpdated:        cfg.lastUpdated,
		Upstream:           cfg.upstream,
	}
}

type userBidCfg struct {
	status       domain.BidStatus
	amount       int
	remaining    *domain.Sats
	remainingSet bool
	upstream     *domain.Upstream
	lastUpdated  time.Time
	pricePerPH   int
	speedPHS     string
}

type UserBidOption func(*userBidCfg)

func WithBidStatus(s domain.BidStatus) UserBidOption {
	return func(c *userBidCfg) { c.status = s }
}

func WithAmount(a int) UserBidOption {
	return func(c *userBidCfg) {
		c.amount = a
		if !c.remainingSet {
			c.remaining = ptrSats(a)
		}
	}
}

func WithRemaining(r int) UserBidOption {
	return func(c *userBidCfg) {
		c.remaining = ptrSats(r)
		c.remainingSet = true
	}
}

func WithUpstream(u domain.Upstream) UserBidOption {
	return func(c *userBidCfg) { c.upstream = &u }
}

func WithLastUpdated(t time.Time) UserBidOption {
	return func(c *userBidCfg) { c.lastUpdated = t }
}

func ptrSats(v int) *domain.Sats {
	s := domain.Sats(v)
	return &s
}

func MakeBidConfig(price int, speed string, identity ...string) domain.BidConfig {
	speedDec, err := decimal.NewFromString(speed)
	if err != nil {
		panic(err)
	}
	slim, err := domain.NewHashrate(speedDec, domain.PH, domain.Second)
	if err != nil {
		panic(err)
	}
	pr, err := domain.NewHashratePrice(domain.Sats(int64(price)), phDay())
	if err != nil {
		panic(err)
	}
	bc := domain.BidConfig{Price: pr, SpeedLimit: slim}
	if len(identity) > 0 {
		bc.Identity = identity[0]
	}
	return bc
}

func MakeSetBidsConfig(up domain.Upstream, bids ...domain.BidConfig) domain.SetBidsConfig {
	return domain.SetBidsConfig{
		DefaultAmount: 100_000,
		Upstream:      up,
		Bids:          bids,
	}
}
