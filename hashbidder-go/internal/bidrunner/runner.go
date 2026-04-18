package bidrunner

import (
	crand "crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/braiins"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"
)

const postExecuteRefetchDelay = 3 * time.Second

type SetBidsResult struct {
	Plan         domain.ReconciliationPlan
	SkippedBids  []domain.UserBid
	BalanceCheck domain.BalanceCheck
	Execution    *ExecutionResult
}

type ActionStatus string

const (
	ActionSucceeded ActionStatus = "succeeded"
	ActionFailed    ActionStatus = "failed"
	ActionSkipped   ActionStatus = "skipped"
)

type ActionOutcome struct {
	Action      any // CancelAction|EditAction|CreateAction
	Status      ActionStatus
	Error       string
	CreatedID   *domain.BidID
	Attempt     *int
	MaxAttempts *int
}

type ExecutionResult struct {
	Outcomes  []ActionOutcome
	FinalBids []domain.UserBid
}

func Reconcile(client braiins.HashpowerClient, cfg domain.SetBidsConfig, dryRun bool, sleep func(time.Duration)) (*SetBidsResult, error) {
	current, err := client.GetCurrentBids()
	if err != nil {
		return nil, err
	}
	plan := domain.PlanBidChanges(cfg, current)
	for _, d := range plan.DeferredCreates {
		slog.Warn("deferred create: existing bid on delivery slot is not ACTIVE/CREATED; will retry on a later run",
			"blocking_bid_id", d.BlockingBid.ID,
			"blocking_status", d.BlockingBid.Status,
			"slot_identity", d.Upstream.Identity,
		)
	}
	skipped := make([]domain.UserBid, 0)
	for _, b := range current {
		if _, ok := domain.ManageableStatuses[b.Status]; !ok {
			skipped = append(skipped, b)
		}
	}
	bal, err := client.GetAccountBalance()
	if err != nil {
		return nil, err
	}
	bc := domain.CheckBalance(plan, bal.AvailableSat)
	if dryRun || bc.Status == domain.BalanceInsufficient {
		return &SetBidsResult{Plan: plan, SkippedBids: skipped, BalanceCheck: bc, Execution: nil}, nil
	}
	ex, err := ExecutePlan(client, plan, sleep)
	if err != nil {
		return nil, err
	}
	return &SetBidsResult{Plan: plan, SkippedBids: skipped, BalanceCheck: bc, Execution: ex}, nil
}

func randomCLOrderID() braiins.ClOrderID {
	var b [16]byte
	_, _ = crand.Read(b[:])
	// UUID v4-ish string for API compatibility with Python's uuid4().
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	s := hex.EncodeToString(b[:])
	return braiins.ClOrderID(fmt.Sprintf("%s-%s-%s-%s-%s", s[0:8], s[8:12], s[12:16], s[16:20], s[20:]))
}

func dispatch(client braiins.HashpowerClient, action any) (*domain.BidID, error) {
	switch a := action.(type) {
	case domain.CancelAction:
		return nil, client.CancelBid(a.Bid.ID)
	case domain.EditAction:
		return nil, client.EditBid(a.Bid.ID, a.NewPrice, a.NewSpeedLimitPH)
	case domain.CreateAction:
		res, err := client.CreateBid(a.Upstream, a.Amount, a.Config.Price, a.Config.SpeedLimit, randomCLOrderID())
		if err != nil {
			return nil, err
		}
		id := res.ID
		return &id, nil
	default:
		return nil, errors.New("unknown action")
	}
}

func executeWithRetries(client braiins.HashpowerClient, action any, outcomes *[]ActionOutcome, sleep func(time.Duration)) bool {
	const maxAttempts = 3
	const retryDelay = 5 * time.Second
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		createdID, err := dispatch(client, action)
		if err == nil {
			*outcomes = append(*outcomes, ActionOutcome{Action: action, Status: ActionSucceeded, CreatedID: createdID})
			return true
		}
		var api *braiins.APIError
		if errors.As(err, &api) && api.IsTransient() && attempt < maxAttempts {
			att := attempt
			max := maxAttempts
			*outcomes = append(*outcomes, ActionOutcome{
				Action: action, Status: ActionFailed, Error: api.Message,
				Attempt: &att, MaxAttempts: &max,
			})
			sleep(retryDelay)
			continue
		}
		msg := err.Error()
		var attPtr *int
		var maxPtr *int
		if errors.As(err, &api) && api.IsTransient() {
			att := attempt
			max := maxAttempts
			attPtr = &att
			maxPtr = &max
		}
		*outcomes = append(*outcomes, ActionOutcome{
			Action: action, Status: ActionFailed, Error: msg,
			Attempt: attPtr, MaxAttempts: maxPtr,
		})
		return false
	}
	return false
}

func ExecutePlan(client braiins.HashpowerClient, plan domain.ReconciliationPlan, sleep func(time.Duration)) (*ExecutionResult, error) {
	outcomes := []ActionOutcome{}
	failedCancel := map[domain.BidID]struct{}{}

	for _, cancel := range plan.Cancels {
		ok := executeWithRetries(client, cancel, &outcomes, sleep)
		if !ok && cancel.Reason == domain.CancelReasonUpstreamMismatch {
			failedCancel[cancel.Bid.ID] = struct{}{}
		}
	}
	for _, edit := range plan.Edits {
		executeWithRetries(client, edit, &outcomes, sleep)
	}
	for _, create := range plan.Creates {
		if create.Replaces != nil {
			if _, ok := failedCancel[create.Replaces.ID]; ok {
				outcomes = append(outcomes, ActionOutcome{Action: create, Status: ActionSkipped})
				continue
			}
		}
		executeWithRetries(client, create, &outcomes, sleep)
	}

	if len(plan.Cancels)+len(plan.Edits)+len(plan.Creates) > 0 {
		sleep(postExecuteRefetchDelay)
	}
	final, err := client.GetCurrentBids()
	if err != nil {
		return nil, err
	}
	return &ExecutionResult{Outcomes: outcomes, FinalBids: final}, nil
}
