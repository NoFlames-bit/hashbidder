package bidrunner

import (
	"strings"
	"testing"
	"time"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/braiins"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/testhelpers"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/testutil"
)

func noSleep(time.Duration) {}

func TestExecutePlan_RetriesTransient(t *testing.T) {
	bid := testhelpers.MakeUserBid("B1", 500, "5.0")
	errs := map[string][]*braiins.APIError{
		"cancel_bid:B1": {{StatusCode: 429, Message: "rate limited"}},
	}
	c := testutil.NewFakeClient(testutil.WithCurrentBids(bid), testutil.WithErrors(errs))
	cfg := testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool)
	plan := domain.PlanBidChanges(cfg, []domain.UserBid{bid})
	res, err := ExecutePlan(c, plan, noSleep)
	if err != nil {
		t.Fatal(err)
	}
	var nonSkip int
	for _, o := range res.Outcomes {
		if o.Status != ActionSkipped {
			nonSkip++
		}
	}
	if nonSkip != 2 {
		t.Fatalf("outcomes=%+v", res.Outcomes)
	}
	cur, _ := c.GetCurrentBids()
	if len(cur) != 0 {
		t.Fatalf("expected empty bids")
	}
}

func TestExecutePlan_PermanentNoRetry(t *testing.T) {
	bid := testhelpers.MakeUserBid("B1", 500, "5.0")
	errs := map[string][]*braiins.APIError{"cancel_bid:B1": {{StatusCode: 400, Message: "bad request"}}}
	c := testutil.NewFakeClient(testutil.WithCurrentBids(bid), testutil.WithErrors(errs))
	cfg := testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool)
	plan := domain.PlanBidChanges(cfg, []domain.UserBid{bid})
	res, err := ExecutePlan(c, plan, noSleep)
	if err != nil {
		t.Fatal(err)
	}
	failed := 0
	for _, o := range res.Outcomes {
		if o.Status == ActionFailed {
			failed++
			if o.Attempt != nil {
				t.Fatalf("attempt should be nil for permanent errors")
			}
			if !strings.Contains(o.Error, "bad request") {
				t.Fatalf("msg=%q", o.Error)
			}
		}
	}
	if failed != 1 {
		t.Fatalf("failed=%d outcomes=%+v", failed, res.Outcomes)
	}
}

func TestExecutePlan_ExhaustRetries(t *testing.T) {
	bid := testhelpers.MakeUserBid("B1", 500, "5.0")
	errs := map[string][]*braiins.APIError{
		"cancel_bid:B1": {
			{StatusCode: 500, Message: "internal error"},
			{StatusCode: 500, Message: "internal error"},
			{StatusCode: 500, Message: "internal error"},
		},
	}
	c := testutil.NewFakeClient(testutil.WithCurrentBids(bid), testutil.WithErrors(errs))
	cfg := testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool)
	plan := domain.PlanBidChanges(cfg, []domain.UserBid{bid})
	res, err := ExecutePlan(c, plan, noSleep)
	if err != nil {
		t.Fatal(err)
	}
	failed := 0
	for _, o := range res.Outcomes {
		if o.Status == ActionFailed {
			failed++
		}
	}
	if failed != 3 {
		t.Fatalf("failed=%d %+v", failed, res.Outcomes)
	}
}

func TestReconcile_InsufficientAborts(t *testing.T) {
	c := testutil.NewFakeClient(
		testutil.WithAccountBalance(braiins.AccountBalance{AvailableSat: 50_000, BlockedSat: 0, TotalSat: 50_000}),
	)
	cfg := testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool, testhelpers.MakeBidConfig(500, "5.0"))
	res, err := Reconcile(c, cfg, false, noSleep)
	if err != nil {
		t.Fatal(err)
	}
	if res.Execution != nil || res.BalanceCheck.Status != domain.BalanceInsufficient {
		t.Fatalf("bad %+v", res)
	}
	var mut int
	for _, call := range c.Calls {
		if len(call) > 0 && (call[0] == "create_bid" || call[0] == "edit_bid" || call[0] == "cancel_bid") {
			mut++
		}
	}
	if mut != 0 {
		t.Fatalf("mutations=%d", mut)
	}
}

func TestReconcile_LowStillExecutes(t *testing.T) {
	c := testutil.NewFakeClient(
		testutil.WithAccountBalance(braiins.AccountBalance{AvailableSat: 9_000_000 * 71, BlockedSat: 0, TotalSat: 9_000_000 * 71}),
	)
	cfg := testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool, testhelpers.MakeBidConfig(500, "5.0"))
	res, err := Reconcile(c, cfg, false, noSleep)
	if err != nil {
		t.Fatal(err)
	}
	if res.BalanceCheck.Status != domain.BalanceLow || res.Execution == nil {
		t.Fatalf("bad %+v", res.BalanceCheck)
	}
	found := false
	for _, call := range c.Calls {
		if len(call) > 0 && call[0] == "create_bid" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected create")
	}
}

func TestExecutePlan_RefetchSleep(t *testing.T) {
	bid := testhelpers.MakeUserBid("B1", 500, "5.0", testhelpers.WithUpstream(testhelpers.OtherUpstream))
	c := testutil.NewFakeClient(testutil.WithCurrentBids(bid))
	cfg := testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool)
	plan := domain.PlanBidChanges(cfg, []domain.UserBid{bid})
	var sleeps []time.Duration
	_, err := ExecutePlan(c, plan, func(d time.Duration) { sleeps = append(sleeps, d) })
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range sleeps {
		if d == postExecuteRefetchDelay {
			found = true
		}
	}
	if !found {
		t.Fatalf("sleeps=%v", sleeps)
	}
}

func TestExecutePlan_NoSleepEmptyPlan(t *testing.T) {
	c := testutil.NewFakeClient()
	cfg := testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool)
	plan := domain.PlanBidChanges(cfg, nil)
	var sleeps []time.Duration
	_, err := ExecutePlan(c, plan, func(d time.Duration) { sleeps = append(sleeps, d) })
	if err != nil {
		t.Fatal(err)
	}
	if len(sleeps) != 0 {
		t.Fatalf("sleeps=%v", sleeps)
	}
}

func TestExecutePlan_FailFirstCreate(t *testing.T) {
	bid := testhelpers.MakeUserBid("B1", 500, "5.0", testhelpers.WithUpstream(testhelpers.OtherUpstream))
	c := testutil.NewFakeClient(testutil.WithCurrentBids(bid))
	c.FailFirstCreate = true
	cfg := testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool, testhelpers.MakeBidConfig(500, "5.0"))
	plan := domain.PlanBidChanges(cfg, []domain.UserBid{bid})
	res, err := ExecutePlan(c, plan, noSleep)
	if err != nil {
		t.Fatal(err)
	}
	ok := false
	for _, o := range res.Outcomes {
		if o.Status == ActionFailed && strings.Contains(strings.ToLower(o.Error), "insufficient balance") {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("outcomes=%+v", res.Outcomes)
	}
}
