package domain

type BidConfig struct {
	Price      HashratePrice
	SpeedLimit Hashrate
}

type SetBidsConfig struct {
	DefaultAmount Sats
	Upstream      Upstream
	Bids          []BidConfig
}
