package domain

import "time"

type BidID string

type BidStatus string

const (
	BidStatusUnspecified   BidStatus = "BID_STATUS_UNSPECIFIED"
	BidStatusActive        BidStatus = "BID_STATUS_ACTIVE"
	BidStatusPendingCancel BidStatus = "BID_STATUS_PENDING_CANCEL"
	BidStatusCanceled      BidStatus = "BID_STATUS_CANCELED"
	BidStatusFulfilled     BidStatus = "BID_STATUS_FULFILLED"
	BidStatusPaused        BidStatus = "BID_STATUS_PAUSED"
	BidStatusFrozen        BidStatus = "BID_STATUS_FROZEN"
	BidStatusCreated       BidStatus = "BID_STATUS_CREATED"
)

var ManageableStatuses = map[BidStatus]struct{}{
	BidStatusActive:  {},
	BidStatusCreated: {},
}

// IsTerminalBidOrderStatus reports whether the bid is finished and no longer
// occupies a live slot for the worker (so a new create is appropriate).
func IsTerminalBidOrderStatus(s BidStatus) bool {
	switch s {
	case BidStatusCanceled, BidStatusFulfilled:
		return true
	default:
		return false
	}
}

type UserBid struct {
	ID                 BidID
	Price              HashratePrice
	SpeedLimitPH       Hashrate
	AmountSat          Sats
	Status             BidStatus
	Progress           *Progress
	AmountRemainingSat *Sats
	LastUpdated        time.Time
	Upstream           *Upstream
}
