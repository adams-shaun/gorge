package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// The second leg's beneficiary rewrite must not leak into the parsed card:
// the registry shares one *cards.Card across pools, so a second lowering of
// the same SA has to see the printed referent and emit its guard again.
func TestAttackLegLoweringDoesNotMutateTheCard(t *testing.T) {
	lc := &lowerCtx{ab: &lAbility{Trigger: &lTrigger{Event: "AttackersDeclared", DefPlayer: 4}}}
	sa := &cards.SA{API: "Token", Params: map[string]string{"TokenOwner": kccLeg}}
	for round := 1; round <= 2; round++ {
		before := len(lc.ab.Prog)
		guard, err := lc.lowerAttackLeg(sa, newParams(sa.Params))
		if err != nil {
			t.Fatalf("round %d: %v", round, err)
		}
		if guard != before || len(lc.ab.Prog) != before+1 {
			t.Fatalf("round %d: guard=%d prog %d->%d, want the SkipIf guard emitted", round, guard, before, len(lc.ab.Prog))
		}
		if got := sa.Params["TokenOwner"]; got != kccLeg {
			t.Fatalf("round %d: the parsed card's TokenOwner$ became %q", round, got)
		}
	}
}
