package formatter

import (
	"fmt"
	"strings"
	"time"

	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/bidrunner"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/domain"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/hv"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/ocean"
	"github.com/NoFlames-bit/hashbidder/hashbidder-go/internal/targethr"

	"github.com/shopspring/decimal"
)

func fmtSpeed(value decimal.Decimal) string {
	if value.Sub(value.Truncate(0)).IsZero() {
		return fmt.Sprintf("%.1f", mustFloat(value))
	}
	return value.String()
}

func mustFloat(d decimal.Decimal) float64 {
	f, _ := d.Float64()
	return f
}

func toPHDay(price domain.HashratePrice) domain.Sats {
	return price.To(domain.PH, domain.Day).Sats
}

func formatEdit(edit domain.EditAction) string {
	oldPrice := toPHDay(edit.OldPrice)
	newPrice := toPHDay(edit.NewPrice)
	var priceLine string
	if edit.PriceChanged() {
		priceLine = fmt.Sprintf("  price:       %d → %d sat/PH/Day", oldPrice, newPrice)
	} else {
		priceLine = fmt.Sprintf("  price:       %d sat/PH/Day (unchanged)", oldPrice)
	}
	var speedLine string
	if edit.SpeedLimitChanged() {
		speedLine = fmt.Sprintf("  speed_limit: %s → %s PH/s",
			fmtSpeed(edit.OldSpeedLimitPH.Value), fmtSpeed(edit.NewSpeedLimitPH.Value))
	} else {
		speedLine = fmt.Sprintf("  speed_limit: %s PH/s (unchanged)", fmtSpeed(edit.OldSpeedLimitPH.Value))
	}
	lines := []string{
		fmt.Sprintf("EDIT %s:", edit.Bid.ID),
		priceLine,
		speedLine,
		"  upstream:    (unchanged)",
	}
	return strings.Join(lines, "\n")
}

func formatCreate(create domain.CreateAction) string {
	price := toPHDay(create.Config.Price)
	speed := fmtSpeed(create.Config.SpeedLimit.Value)
	header := "CREATE:"
	if create.Replaces != nil {
		header = fmt.Sprintf("CREATE (replaces %s):", create.Replaces.ID)
	}
	lines := []string{
		header,
		fmt.Sprintf("  price:       %d sat/PH/Day", price),
		fmt.Sprintf("  speed_limit: %s PH/s", speed),
		fmt.Sprintf("  amount:      %d sat", create.Amount),
		fmt.Sprintf("  upstream:    %s / %s", create.Upstream.URL.String(), create.Upstream.Identity),
	}
	return strings.Join(lines, "\n")
}

func formatCancel(cancel domain.CancelAction) string {
	price := toPHDay(cancel.Bid.Price)
	speed := fmtSpeed(cancel.Bid.SpeedLimitPH.Value)
	lines := []string{
		fmt.Sprintf("CANCEL %s:", cancel.Bid.ID),
		fmt.Sprintf("  price:       %d sat/PH/Day", price),
		fmt.Sprintf("  speed_limit: %s PH/s", speed),
		fmt.Sprintf("  reason:      %s", cancel.Reason),
	}
	return strings.Join(lines, "\n")
}

func formatFinalStateLine(pricePHDay domain.Sats, speed string, amount domain.Sats, annotation string) string {
	return fmt.Sprintf("BID  price=%d sat/PH/Day  limit=%s PH/s  amount=%d sat  (%s)", pricePHDay, speed, amount, annotation)
}

func formatDeferredCreate(d domain.DeferredCreate) string {
	price := toPHDay(d.Config.Price)
	speed := fmtSpeed(d.Config.SpeedLimit.Value)
	return fmt.Sprintf(
		"DEFER create: target %d sat/PH/Day %s PH/s — existing bid %s is %s (not ACTIVE/CREATED); skipping duplicate order until next run",
		price, speed, d.BlockingBid.ID, d.BlockingBid.Status,
	)
}

func FormatPlan(plan domain.ReconciliationPlan, skipped []domain.UserBid) string {
	sections := []string{}
	hasChanges := len(plan.Edits) > 0 || len(plan.Creates) > 0 || len(plan.Cancels) > 0 || len(plan.DeferredCreates) > 0
	if !hasChanges {
		sections = append(sections, "No changes needed.")
	} else {
		sections = append(sections, "=== Changes ===")
		replacement := map[domain.BidID]domain.CreateAction{}
		for _, cr := range plan.Creates {
			if cr.Replaces != nil {
				replacement[cr.Replaces.ID] = cr
			}
		}
		for _, edit := range plan.Edits {
			sections = append(sections, formatEdit(edit))
		}
		for _, cancel := range plan.Cancels {
			sections = append(sections, formatCancel(cancel))
			if cancel.Reason == domain.CancelReasonUpstreamMismatch {
				sections = append(sections, formatCreate(replacement[cancel.Bid.ID]))
			}
		}
		for _, create := range plan.Creates {
			if create.Replaces == nil {
				sections = append(sections, formatCreate(create))
			}
		}
		for _, def := range plan.DeferredCreates {
			sections = append(sections, formatDeferredCreate(def))
		}
	}

	stateLines := []string{}
	for _, edit := range plan.Edits {
		price := toPHDay(edit.NewPrice)
		speed := fmtSpeed(edit.NewSpeedLimitPH.Value)
		changes := []string{}
		if edit.PriceChanged() {
			changes = append(changes, fmt.Sprintf("price %d→%d", toPHDay(edit.OldPrice), price))
		}
		if edit.SpeedLimitChanged() {
			changes = append(changes, fmt.Sprintf("speed_limit %s→%s",
				fmtSpeed(edit.OldSpeedLimitPH.Value), fmtSpeed(edit.NewSpeedLimitPH.Value)))
		}
		stateLines = append(stateLines, formatFinalStateLine(price, speed, edit.Bid.AmountSat, "EDITED, "+strings.Join(changes, ", ")))
	}
	for _, create := range plan.Creates {
		price := toPHDay(create.Config.Price)
		speed := fmtSpeed(create.Config.SpeedLimit.Value)
		stateLines = append(stateLines, formatFinalStateLine(price, speed, create.Amount, "NEW"))
	}
	for _, def := range plan.DeferredCreates {
		price := toPHDay(def.Config.Price)
		speed := fmtSpeed(def.Config.SpeedLimit.Value)
		stateLines = append(stateLines, formatFinalStateLine(price, speed, def.Amount,
			fmt.Sprintf("DEFERRED until bid %s is ACTIVE/CREATED (currently %s)", def.BlockingBid.ID, def.BlockingBid.Status)))
	}
	for _, unch := range plan.Unchanged {
		price := toPHDay(unch.Bid.Price)
		speed := fmtSpeed(unch.Bid.SpeedLimitPH.Value)
		stateLines = append(stateLines, formatFinalStateLine(price, speed, unch.Bid.AmountSat, "UNCHANGED"))
	}
	for _, bid := range skipped {
		price := toPHDay(bid.Price)
		speed := fmtSpeed(bid.SpeedLimitPH.Value)
		stateLines = append(stateLines, formatFinalStateLine(price, speed, bid.AmountSat, string(bid.Status)))
	}

	sections = append(sections, "")
	sections = append(sections, "=== Expected Final State ===")
	if len(stateLines) > 0 {
		sections = append(sections, stateLines...)
	} else {
		sections = append(sections, "No active bids.")
	}
	return strings.Join(sections, "\n")
}

func actionLabel(a any) string {
	switch v := a.(type) {
	case domain.CancelAction:
		return fmt.Sprintf("CANCEL %s", v.Bid.ID)
	case domain.EditAction:
		return fmt.Sprintf("EDIT %s", v.Bid.ID)
	case domain.CreateAction:
		return fmt.Sprintf("CREATE %d sat/PH/Day %s PH/s", toPHDay(v.Config.Price), fmtSpeed(v.Config.SpeedLimit.Value))
	default:
		return "ACTION"
	}
}

func FormatOutcome(o bidrunner.ActionOutcome) string {
	label := actionLabel(o.Action)
	switch o.Status {
	case bidrunner.ActionSucceeded:
		suf := "OK"
		if o.CreatedID != nil {
			suf = fmt.Sprintf("OK → %s", *o.CreatedID)
		}
		return fmt.Sprintf("%s... %s", label, suf)
	case bidrunner.ActionFailed:
		errPart := ""
		if o.Error != "" {
			errPart = ": " + o.Error
		}
		attPart := ""
		if o.Attempt != nil && o.MaxAttempts != nil {
			attPart = fmt.Sprintf(" (attempt %d/%d", *o.Attempt, *o.MaxAttempts)
			if *o.Attempt < *o.MaxAttempts {
				attPart += ", retrying in 5s)"
			} else {
				attPart += ")"
			}
		}
		return fmt.Sprintf("%s... FAILED%s%s", label, errPart, attPart)
	default:
		return "  skipping linked CREATE (upstream mismatch pair)"
	}
}

func FormatResultsSummary(outcomes []bidrunner.ActionOutcome) string {
	succeeded, failed, skipped := 0, 0, 0
	for _, o := range outcomes {
		switch o.Status {
		case bidrunner.ActionSucceeded:
			succeeded++
		case bidrunner.ActionFailed:
			failed++
		case bidrunner.ActionSkipped:
			skipped++
		}
	}
	parts := []string{fmt.Sprintf("%d succeeded", succeeded), fmt.Sprintf("%d failed", failed)}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", skipped))
	}
	return strings.Join(parts, ", ")
}

func FormatCurrentBids(bids []domain.UserBid) string {
	if len(bids) == 0 {
		return "No active bids."
	}
	lines := []string{}
	for _, bid := range bids {
		price := toPHDay(bid.Price)
		speed := fmtSpeed(bid.SpeedLimitPH.Value)
		lines = append(lines, fmt.Sprintf("%s  price=%d sat/PH/Day  limit=%s PH/s  amount=%d sat  %s",
			bid.ID, price, speed, bid.AmountSat, bid.Status))
	}
	return strings.Join(lines, "\n")
}

func FormatOceanStats(stats ocean.AccountStats, addr domain.BtcAddress) string {
	allZero := true
	for _, w := range stats.Windows {
		if !w.Hashrate.Value.IsZero() {
			allZero = false
			break
		}
	}
	if allZero {
		return fmt.Sprintf("No stats found for %s on Ocean.", addr.String())
	}
	lines := []string{fmt.Sprintf("Ocean stats for %s", addr.Truncated()), ""}
	for _, w := range stats.Windows {
		disp := w.Hashrate.DisplayUnit()
		label := string(w.Window)
		lines = append(lines, fmt.Sprintf("  %6s    %.2f %s/s", label, mustFloat(disp.Value), disp.HashUnit.Name()))
	}
	return strings.Join(lines, "\n")
}

func FormatTargetInputsHashrate(ocean24h, target, needed domain.Hashrate, price domain.HashratePrice) string {
	oceanPH := ocean24h.To(domain.PH, domain.Second).Value
	targetPH := target.To(domain.PH, domain.Second).Value
	neededPH := needed.To(domain.PH, domain.Second).Value
	pricePHDay := toPHDay(price)
	lines := []string{
		"=== Target Hashrate Inputs ===",
		fmt.Sprintf("  Ocean 24h:    %s PH/s", fmtSpeed(oceanPH)),
		fmt.Sprintf("  Target:       %s PH/s", fmtSpeed(targetPH)),
		fmt.Sprintf("  Needed:       %s PH/s", fmtSpeed(neededPH)),
		fmt.Sprintf("  Market price: %d sat/PH/Day", pricePHDay),
	}
	return strings.Join(lines, "\n")
}

func fmtRunway(bc domain.BalanceCheck) string {
	if bc.RunwayInfinite {
		return "∞"
	}
	hours := decimal.NewFromInt(bc.Runway.Nanoseconds()).Div(decimal.NewFromInt(int64(time.Hour)))
	return hours.StringFixed(1) + "h"
}

func FormatBalanceCheck(check domain.BalanceCheck) string {
	thresholdHours := int(domain.LowBalanceRunway.Hours())
	br := check.BurnRate.To(time.Hour)
	burnPerHour := br.Amount
	lines := []string{
		"=== Account Balance ===",
		fmt.Sprintf("  Available:  %s sat", formatInt(int64(check.AvailableSat))),
		fmt.Sprintf("  Required:   %s sat", formatInt(int64(check.RequiredSat))),
		fmt.Sprintf("  Burn rate:  %s sat/hour", burnPerHour.StringFixed(0)),
		fmt.Sprintf("  Runway:     %s", fmtRunway(check)),
	}
	switch check.Status {
	case domain.BalanceInsufficient:
		lines = append(lines, "  Status:     INSUFFICIENT — execution aborted")
	case domain.BalanceLow:
		lines = append(lines, fmt.Sprintf("  Status:     LOW — runway under %dh", thresholdHours))
	default:
		lines = append(lines, "  Status:     SUFFICIENT")
	}
	return strings.Join(lines, "\n")
}

func formatInt(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	sign := ""
	if s[0] == '-' {
		sign = "-"
		s = s[1:]
	}
	out := ""
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out += ","
		}
		out += string(c)
	}
	return sign + out
}

func FormatSetBidsResult(res *bidrunner.SetBidsResult) string {
	plan := res.Plan
	hasChanges := len(plan.Edits) > 0 || len(plan.Creates) > 0 || len(plan.Cancels) > 0 || len(plan.DeferredCreates) > 0
	bal := FormatBalanceCheck(res.BalanceCheck)

	if res.BalanceCheck.Status == domain.BalanceInsufficient {
		return strings.Join([]string{
			bal,
			"",
			"Execution aborted: insufficient balance to fund planned creates.",
			"",
			FormatPlan(plan, res.SkippedBids),
		}, "\n")
	}
	if res.Execution == nil {
		return strings.Join([]string{bal, "", FormatPlan(plan, res.SkippedBids)}, "\n")
	}
	if !hasChanges {
		return strings.Join([]string{bal, "", "No changes needed."}, "\n")
	}
	sections := []string{bal, ""}
	if len(plan.DeferredCreates) > 0 {
		sections = append(sections, FormatPlan(plan, res.SkippedBids), "")
	}
	if len(res.Execution.Outcomes) == 0 {
		sections = append(sections, "No API mutations executed (deferred create(s) withheld).")
		return strings.Join(sections, "\n")
	}
	sections = append(sections, "=== Executing Changes ===")
	for _, o := range res.Execution.Outcomes {
		sections = append(sections, FormatOutcome(o))
	}
	sections = append(sections, "", "=== Results ===", FormatResultsSummary(res.Execution.Outcomes), "", "=== Current Bids ===", FormatCurrentBids(res.Execution.FinalBids))
	return strings.Join(sections, "\n")
}

type SetBidsTargetResult struct {
	Inputs        TargetHashrateInputsView
	SetBidsResult *bidrunner.SetBidsResult
}

type TargetHashrateInputsView struct {
	Ocean24h      domain.Hashrate
	Target        domain.Hashrate
	Needed        domain.Hashrate
	Price         domain.HashratePrice
	MaxBidsCount  int
	AnnotatedBids []targethr.BidWithCooldown
}

func FormatSetBidsTargetResult(r SetBidsTargetResult) string {
	return strings.Join([]string{
		FormatTargetInputsHashrate(r.Inputs.Ocean24h, r.Inputs.Target, r.Inputs.Needed, r.Inputs.Price),
		"",
		FormatSetBidsResult(r.SetBidsResult),
	}, "\n")
}

func FormatSetBidsTargetResultVerbose(r SetBidsTargetResult) string {
	sections := []string{
		FormatTargetInputsHashrate(r.Inputs.Ocean24h, r.Inputs.Target, r.Inputs.Needed, r.Inputs.Price),
		"",
		formatTargetDistributionMath(r.Inputs),
		"",
		formatTargetCooldowns(r.Inputs.AnnotatedBids),
		"",
		FormatSetBidsResult(r.SetBidsResult),
	}
	return strings.Join(sections, "\n")
}

func formatTargetDistributionMath(in TargetHashrateInputsView) string {
	targetPH := in.Target.To(domain.PH, domain.Second).Value
	oceanPH := in.Ocean24h.To(domain.PH, domain.Second).Value
	neededPH := in.Needed.To(domain.PH, domain.Second).Value
	pricePHDay := toPHDay(in.Price)
	served := int(pricePHDay) - 1
	lines := []string{
		"=== Reasoning ===",
		fmt.Sprintf("  Price scan:   lowest served bid %d sat/PH/Day → undercut by 1 sat → %d sat/PH/Day", served, pricePHDay),
		fmt.Sprintf("  Needed math:  2 * %s (target) - %s (ocean 24h) = %s PH/s", fmtSpeed(targetPH), fmtSpeed(oceanPH), fmtSpeed(neededPH)),
		fmt.Sprintf("  Slot budget:  up to %d bids (min 1 PH/s each, quantized to 0.01 PH/s)", in.MaxBidsCount),
	}
	return strings.Join(lines, "\n")
}

func formatTargetCooldowns(annotated []targethr.BidWithCooldown) string {
	lines := []string{"=== Cooldown Status ==="}
	if len(annotated) == 0 {
		lines = append(lines, "  (no existing bids)")
		return strings.Join(lines, "\n")
	}
	for _, entry := range annotated {
		cd := entry.Cooldown
		var status string
		switch {
		case cd.PriceCooldown && cd.SpeedCooldown:
			status = "price+speed locked"
		case cd.PriceCooldown:
			status = "price locked (speed free)"
		case cd.SpeedCooldown:
			status = "speed locked (price free)"
		default:
			status = "free"
		}
		price := toPHDay(entry.Bid.Price)
		lines = append(lines, fmt.Sprintf("  %s  price=%d sat/PH/Day  limit=%s  → %s", entry.Bid.ID, price, entry.Bid.SpeedLimitPH.String(), status))
	}
	return strings.Join(lines, "\n")
}

func FormatHashvalue(c hv.Components) string {
	return fmt.Sprintf("Hashvalue: %d sat/PH/Day", c.Hashvalue.Sats)
}

func FormatHashvalueVerbose(c hv.Components, mempoolURL string) string {
	lines := []string{
		FormatHashvalue(c),
		"",
		fmt.Sprintf("  Tip height:       %s", c.TipHeight.String()),
		fmt.Sprintf("  Block subsidy:    %d sat", c.Subsidy),
		fmt.Sprintf("  Total fees (2016): %d sat", c.TotalFees),
		fmt.Sprintf("  Total reward (2016): %d sat", c.TotalReward),
		fmt.Sprintf("  Difficulty:       %s", c.Difficulty.String()),
		fmt.Sprintf("  Network hashrate: %.2E H/s", mustFloat(c.NetworkHashrate)),
		fmt.Sprintf("  Mempool instance: %s", mempoolURL),
	}
	return strings.Join(lines, "\n")
}
