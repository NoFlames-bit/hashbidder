package formatter

import (
	"strings"
	"testing"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/hv"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/testhelpers"

	"github.com/shopspring/decimal"
)

func TestFormatHashvalue(t *testing.T) {
	tip, _ := domain.NewBlockHeight(840_000)
	pr, _ := domain.NewHashratePrice(domain.Sats(123), mustPerPHDay())
	c := hv.Components{TipHeight: tip, Hashvalue: pr}
	out := FormatHashvalue(c)
	if out != "Hashvalue: 123 sat/PH/Day" {
		t.Fatalf("got %q", out)
	}
}

func mustPerPHDay() domain.Hashrate {
	h, err := domain.NewHashrate(decimal.NewFromInt(1), domain.PH, domain.Day)
	if err != nil {
		panic(err)
	}
	return h
}

func TestFormatPlan_NoChanges(t *testing.T) {
	plan := domain.PlanBidChanges(testhelpers.MakeSetBidsConfig(testhelpers.UpstreamPool), nil)
	out := FormatPlan(plan, nil)
	if !strings.Contains(out, "No changes needed") {
		t.Fatalf("got %q", out)
	}
}
