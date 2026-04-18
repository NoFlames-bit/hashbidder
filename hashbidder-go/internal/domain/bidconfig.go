package domain

type BidConfig struct {
	Price      HashratePrice
	SpeedLimit Hashrate
	// Identity overrides [upstream].identity for this row when non-empty (same stratum URL).
	Identity string
}

type SetBidsConfig struct {
	DefaultAmount Sats
	Upstream      Upstream
	Bids          []BidConfig
}
