package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestConditionBareThresholdCountsTheGraveyard proves the bare
// `Condition$ Threshold` gate resolves: met exactly with seven or more cards
// in the controller's graveyard, and unmet (but resolved) for a controller
// the game has no seat for.
func TestConditionBareThresholdCountsTheGraveyard(t *testing.T) {
	gate := sa(t, "DB$ Draw | Condition$ Threshold")
	for _, tc := range []struct {
		n    int
		ctrl state.PlayerID
		want bool
	}{
		{6, 0, false},
		{7, 0, true},
		{7, 9, false},
	} {
		h, ids := conditionBoard(t)
		g := h.g
		filler := mkCard(t, "Name:Filler\nTypes:Instant\nOracle:x\n")
		for range tc.n {
			o := g.AddObject(filler, 0)
			o.Zone = state.ZGraveyard
			g.SetZone(state.ZGraveyard, 0, append(g.Zone(state.ZGraveyard, 0), o.ID))
		}
		// Precondition: the zone the rule reads holds exactly the cards put there.
		if got := g.CardCount(state.ZGraveyard, 0); got != tc.n {
			t.Fatalf("n=%d: graveyard holds %d cards, fixture is wrong", tc.n, got)
		}
		ctx := &Ctx{Controller: tc.ctrl, Source: ids[3]}
		met, resolved := conditionMet(h, ctx, gate)
		if !resolved || met != tc.want {
			t.Fatalf("n=%d ctrl=%d: met=%v resolved=%v, want %v true", tc.n, tc.ctrl, met, resolved, tc.want)
		}
	}
}
