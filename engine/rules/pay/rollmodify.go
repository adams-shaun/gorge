package pay

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// rollmodify.go prices and pays the cost of a die-roll-modify keyword (Night
// Shift of the Living Dead's RollModifyCost SVar, CR 706.2). Only the fixed
// PayLife<N> shape is modelled; any other cost is neither payable nor paid.

// rollModifyLife extracts N from a "PayLife<N>" cost (bare digits, N >= 1).
func rollModifyLife(cost string) (int32, bool) {
	rest, ok := strings.CutPrefix(strings.TrimSpace(cost), "PayLife<")
	if !ok {
		return 0, false
	}
	inner, ok := strings.CutSuffix(rest, ">")
	if !ok || inner == "" {
		return 0, false
	}
	for _, r := range inner {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(inner)
	return int32(n), err == nil && n >= 1
}

// RollModifyCostPayable reports whether p can pay cost now: a PayLife<N> with
// life of at least N (CR 119.4) while no static forbids the life payment
// (CR 119.7/119.8).
func RollModifyCostPayable(e Engine, p state.PlayerID, cost string) bool {
	n, ok := rollModifyLife(cost)
	return ok && int(p) < len(e.Game().Players) && !e.Eval().LifeChangeForbidden(p) && e.Game().Players[p].Life >= n
}

// PayRollModifyCost pays cost from p's life through the pay-life proposal
// boundary, so a repl:PayLife replacement still sees the payment. It pays
// nothing and reports false when the cost is not payable.
func PayRollModifyCost(e Engine, p state.PlayerID, cost string) bool {
	if !RollModifyCostPayable(e, p, cost) {
		return false
	}
	n, _ := rollModifyLife(cost)
	e.Emit(events.Event{Kind: events.LifeChange, Player: p, Amount: -n, Text: PayLifeProposalText})
	return true
}
