package domain_test

import (
	"math/rand"
	"strconv"
	"testing"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/testhelpers"
)

func TestPlanBidChanges_Empty(t *testing.T) {
	plan := domain.PlanBidChanges(testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool), nil)
	if len(plan.Edits)+len(plan.Creates)+len(plan.Cancels)+len(plan.Unchanged) != 0 {
		t.Fatalf("expected empty plan, got %+v", plan)
	}
}

func TestPlanBidChanges_ExactMatch(t *testing.T) {
	bid := testhelpers.MakeUserBid("B1", 500, "5.0")
	cfg := testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool, testhelpers.MakeBidConfig(500, "5.0"))
	plan := domain.PlanBidChanges(cfg, []domain.UserBid{bid})
	if len(plan.Unchanged) != 1 || plan.Unchanged[0].Bid.ID != bid.ID {
		t.Fatalf("unexpected unchanged: %+v", plan.Unchanged)
	}
	if len(plan.Edits)+len(plan.Creates)+len(plan.Cancels) != 0 {
		t.Fatalf("expected no other actions")
	}
}

func TestPlanBidChanges_ConfigExtraCreates(t *testing.T) {
	c1 := testhelpers.MakeBidConfig(500, "5.0")
	c2 := testhelpers.MakeBidConfig(300, "10.0")
	plan := domain.PlanBidChanges(testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool, c1, c2), nil)
	if len(plan.Creates) != 2 {
		t.Fatalf("creates=%d", len(plan.Creates))
	}
	if plan.Creates[0].Amount != 100_000 || plan.Creates[0].Replaces != nil {
		t.Fatalf("bad create[0]: %+v", plan.Creates[0])
	}
}

func TestPlanBidChanges_UnmatchedCancels(t *testing.T) {
	bid := testhelpers.MakeUserBid("B1", 500, "5.0")
	plan := domain.PlanBidChanges(testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool), []domain.UserBid{bid})
	if len(plan.Cancels) != 1 || plan.Cancels[0].Reason != domain.CancelReasonUnmatched {
		t.Fatalf("unexpected cancels: %+v", plan.Cancels)
	}
}

func TestPlanBidChanges_PriceEdit(t *testing.T) {
	bid := testhelpers.MakeUserBid("B1", 400, "5.0")
	cfg := testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool, testhelpers.MakeBidConfig(500, "5.0"))
	plan := domain.PlanBidChanges(cfg, []domain.UserBid{bid})
	if len(plan.Edits) != 1 {
		t.Fatalf("edits=%d", len(plan.Edits))
	}
	e := plan.Edits[0]
	if e.Bid.ID != bid.ID || !e.PriceChanged() || e.SpeedLimitChanged() {
		t.Fatalf("unexpected edit %+v", e)
	}
}

func TestPlanBidChanges_SpeedEdit(t *testing.T) {
	bid := testhelpers.MakeUserBid("B1", 500, "3.0")
	cfg := testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool, testhelpers.MakeBidConfig(500, "5.0"))
	plan := domain.PlanBidChanges(cfg, []domain.UserBid{bid})
	e := plan.Edits[0]
	if e.PriceChanged() || !e.SpeedLimitChanged() {
		t.Fatalf("unexpected edit %+v", e)
	}
}

func TestPlanBidChanges_BothEdit(t *testing.T) {
	bid := testhelpers.MakeUserBid("B1", 400, "3.0")
	cfg := testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool, testhelpers.MakeBidConfig(500, "5.0"))
	plan := domain.PlanBidChanges(cfg, []domain.UserBid{bid})
	e := plan.Edits[0]
	if !e.PriceChanged() || !e.SpeedLimitChanged() {
		t.Fatalf("unexpected edit %+v", e)
	}
}

func TestPlanBidChanges_UpstreamMismatch(t *testing.T) {
	bid := testhelpers.MakeUserBid("B1", 500, "5.0", testhelpers.WithUpstream(testhelpers.OtherUpstream))
	cfgEntry := testhelpers.MakeBidConfig(500, "5.0")
	plan := domain.PlanBidChanges(testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool, cfgEntry), []domain.UserBid{bid})
	if len(plan.Cancels) != 1 || plan.Cancels[0].Reason != domain.CancelReasonUpstreamMismatch {
		t.Fatalf("cancels=%+v", plan.Cancels)
	}
	if len(plan.Creates) != 1 || plan.Creates[0].Replaces == nil || plan.Creates[0].Replaces.ID != bid.ID {
		t.Fatalf("creates=%+v", plan.Creates)
	}
}

func TestPlanBidChanges_GreedyRemaining(t *testing.T) {
	bidHigh := testhelpers.MakeUserBid("B1", 500, "3.0", testhelpers.WithRemaining(200_000))
	bidLow := testhelpers.MakeUserBid("B2", 400, "5.0", testhelpers.WithRemaining(10_000))
	cfg := testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool, testhelpers.MakeBidConfig(500, "5.0"))
	plan := domain.PlanBidChanges(cfg, []domain.UserBid{bidLow, bidHigh})
	if len(plan.Edits) != 1 || plan.Edits[0].Bid.ID != bidHigh.ID {
		t.Fatalf("edits=%+v", plan.Edits)
	}
	if len(plan.Cancels) != 1 || plan.Cancels[0].Bid.ID != bidLow.ID {
		t.Fatalf("cancels=%+v", plan.Cancels)
	}
}

func TestPlanBidChanges_FewestDiffsWins(t *testing.T) {
	bid := testhelpers.MakeUserBid("B1", 500, "5.0")
	cExact := testhelpers.MakeBidConfig(500, "5.0")
	cOneOff := testhelpers.MakeBidConfig(600, "5.0")
	plan := domain.PlanBidChanges(testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool, cExact, cOneOff), []domain.UserBid{bid})
	if len(plan.Unchanged) != 1 || len(plan.Creates) != 1 {
		t.Fatalf("unch=%d creates=%d", len(plan.Unchanged), len(plan.Creates))
	}
}

func TestPlanBidChanges_PausedFrozenCanceled(t *testing.T) {
	for _, st := range []domain.BidStatus{domain.BidStatusPaused, domain.BidStatusFrozen} {
		bid := testhelpers.MakeUserBid("B1", 500, "5.0", testhelpers.WithBidStatus(st))
		cfg := testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool, testhelpers.MakeBidConfig(500, "5.0"))
		plan := domain.PlanBidChanges(cfg, []domain.UserBid{bid})
		if len(plan.Creates) != 1 || len(plan.Cancels) != 0 {
			t.Fatalf("status=%s plan=%+v", st, plan)
		}
	}
	bid := testhelpers.MakeUserBid("B1", 500, "5.0", testhelpers.WithBidStatus(domain.BidStatusCanceled))
	plan := domain.PlanBidChanges(testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool, testhelpers.MakeBidConfig(500, "5.0")), []domain.UserBid{bid})
	if len(plan.Creates) != 1 || len(plan.Cancels) != 0 {
		t.Fatalf("canceled plan=%+v", plan)
	}
}

func TestPlanBidChanges_MixedScenario(t *testing.T) {
	bidUn := testhelpers.MakeUserBid("B1", 500, "5.0", testhelpers.WithRemaining(300_000))
	bidEd := testhelpers.MakeUserBid("B2", 400, "10.0", testhelpers.WithRemaining(200_000))
	bidCa := testhelpers.MakeUserBid("B3", 100, "1.0", testhelpers.WithRemaining(50_000))
	bidPz := testhelpers.MakeUserBid("B4", 999, "99.0", testhelpers.WithBidStatus(domain.BidStatusPaused))
	c1 := testhelpers.MakeBidConfig(500, "5.0")
	c2 := testhelpers.MakeBidConfig(300, "10.0")
	plan := domain.PlanBidChanges(
		testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool, c1, c2),
		[]domain.UserBid{bidUn, bidEd, bidCa, bidPz},
	)
	if len(plan.Unchanged) != 1 || plan.Unchanged[0].Bid.ID != bidUn.ID {
		t.Fatalf("unch=%+v", plan.Unchanged)
	}
	if len(plan.Edits) != 1 || plan.Edits[0].Bid.ID != bidEd.ID {
		t.Fatalf("edits=%+v", plan.Edits)
	}
	if len(plan.Cancels) != 1 || plan.Cancels[0].Bid.ID != bidCa.ID {
		t.Fatalf("cancels=%+v", plan.Cancels)
	}
	if len(plan.Creates) != 0 {
		t.Fatalf("creates=%+v", plan.Creates)
	}
}

func TestPlanBidChanges_EmptyConfigCancelsAll(t *testing.T) {
	bids := []domain.UserBid{
		testhelpers.MakeUserBid("B1", 500, "5.0"),
		testhelpers.MakeUserBid("B2", 300, "10.0"),
	}
	plan := domain.PlanBidChanges(testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool), bids)
	if len(plan.Cancels) != 2 {
		t.Fatalf("cancels=%d", len(plan.Cancels))
	}
}

func TestPlanBidChanges_UpstreamMismatchOnEditCandidate(t *testing.T) {
	bid := testhelpers.MakeUserBid("B1", 400, "5.0", testhelpers.WithUpstream(testhelpers.OtherUpstream))
	cfg := testhelpers.MakeBidConfig(500, "5.0")
	plan := domain.PlanBidChanges(testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool, cfg), []domain.UserBid{bid})
	if len(plan.Cancels) != 1 || len(plan.Creates) != 1 {
		t.Fatalf("plan=%+v / %+v", plan.Cancels, plan.Creates)
	}
}

func TestPlanBidChanges_AllPausedCreatesOnly(t *testing.T) {
	bids := []domain.UserBid{
		testhelpers.MakeUserBid("B1", 500, "5.0", testhelpers.WithBidStatus(domain.BidStatusPaused)),
		testhelpers.MakeUserBid("B2", 300, "10.0", testhelpers.WithBidStatus(domain.BidStatusFrozen)),
	}
	c1 := testhelpers.MakeBidConfig(500, "5.0")
	c2 := testhelpers.MakeBidConfig(300, "10.0")
	plan := domain.PlanBidChanges(testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool, c1, c2), bids)
	if len(plan.Creates) != 2 || len(plan.Cancels) != 0 {
		t.Fatalf("plan=%+v", plan)
	}
}

func randBidConfig(r *rand.Rand) domain.BidConfig {
	price := int64(r.Intn(10_000) + 1)
	speeds := []string{"1.0", "5.0", "10.0", "20.0", "50.0"}
	return testhelpers.MakeBidConfig(int(price), speeds[r.Intn(len(speeds))])
}

func randUserBid(r *rand.Rand, id string) domain.UserBid {
	price := r.Intn(10_000) + 1
	speeds := []string{"1.0", "5.0", "10.0", "20.0", "50.0"}
	statuses := []domain.BidStatus{
		domain.BidStatusActive, domain.BidStatusCreated, domain.BidStatusPaused, domain.BidStatusFrozen,
		domain.BidStatusCanceled, domain.BidStatusPendingCancel, domain.BidStatusFulfilled,
	}
	st := statuses[r.Intn(len(statuses))]
	rem := r.Intn(1_000_000) + 1
	up := testhelpers.UpstreamPool
	if r.Intn(2) == 0 {
		up = testhelpers.OtherUpstream
	}
	return testhelpers.MakeUserBid(id, price, speeds[r.Intn(len(speeds))],
		testhelpers.WithBidStatus(st),
		testhelpers.WithRemaining(rem),
		testhelpers.WithUpstream(up),
	)
}

func TestPlanBidChanges_RandomInvariants(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for iter := 0; iter < 200; iter++ {
		nCfg := r.Intn(7)
		nBid := r.Intn(7)
		bids := make([]domain.UserBid, nBid)
		for i := 0; i < nBid; i++ {
			bids[i] = randUserBid(r, "B"+strconv.Itoa(i))
		}
		cfgs := make([]domain.BidConfig, nCfg)
		for i := range cfgs {
			cfgs[i] = randBidConfig(r)
		}
		up := testhelpers.UpstreamPool
		if r.Intn(2) == 0 {
			up = testhelpers.OtherUpstream
		}
		cfg := testhelpers.MakeSetBidsConfig(up, cfgs...)
		plan := domain.PlanBidChanges(cfg, bids)

		edited := map[domain.BidID]struct{}{}
		unch := map[domain.BidID]struct{}{}
		canceled := map[domain.BidID]struct{}{}
		for _, e := range plan.Edits {
			edited[e.Bid.ID] = struct{}{}
		}
		for _, u := range plan.Unchanged {
			unch[u.Bid.ID] = struct{}{}
		}
		for _, c := range plan.Cancels {
			canceled[c.Bid.ID] = struct{}{}
		}
		manageable := 0
		for _, b := range bids {
			if _, ok := domain.ManageableStatuses[b.Status]; ok {
				manageable++
				cnt := 0
				if _, ok := edited[b.ID]; ok {
					cnt++
				}
				if _, ok := unch[b.ID]; ok {
					cnt++
				}
				if _, ok := canceled[b.ID]; ok {
					cnt++
				}
				if cnt != 1 {
					t.Fatalf("iter=%d bid %s in %d buckets", iter, b.ID, cnt)
				}
			}
		}
		if len(edited)+len(unch)+len(canceled) != manageable {
			t.Fatalf("iter=%d bucket sizes mismatch", iter)
		}

		for _, b := range bids {
			if _, ok := domain.ManageableStatuses[b.Status]; ok {
				continue
			}
			if _, ok := edited[b.ID]; ok {
				t.Fatalf("iter=%d non-manageable in edits", iter)
			}
			if _, ok := unch[b.ID]; ok {
				t.Fatalf("iter=%d non-manageable in unchanged", iter)
			}
			if _, ok := canceled[b.ID]; ok {
				t.Fatalf("iter=%d non-manageable in cancels", iter)
			}
		}

		mismatch := 0
		for _, c := range plan.Cancels {
			if c.Reason == domain.CancelReasonUpstreamMismatch {
				mismatch++
			}
		}
		repl := 0
		for _, cr := range plan.Creates {
			if cr.Replaces != nil {
				repl++
			}
		}
		if mismatch != repl {
			t.Fatalf("iter=%d mismatch=%d repl=%d", iter, mismatch, repl)
		}

		matched := len(plan.Edits) + len(plan.Unchanged) + mismatch
		pureCreates := len(plan.Creates) - mismatch
		if matched+pureCreates != len(cfg.Bids) {
			t.Fatalf("iter=%d cfg accounting matched=%d pure=%d bids=%d", iter, matched, pureCreates, len(cfg.Bids))
		}

		for _, cr := range plan.Creates {
			if cr.Amount != cfg.DefaultAmount {
				t.Fatalf("iter=%d create amount", iter)
			}
		}
	}
}
