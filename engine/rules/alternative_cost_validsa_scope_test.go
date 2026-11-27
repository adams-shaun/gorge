package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestAlternativeCostValidSASpellConstraintFailsClosed pins the fail-closed
// direction for an unevaluable ValidSA$ Spell constraint: the free cast is
// denied, never silently widened into a paid cast.
func TestAlternativeCostValidSASpellConstraintFailsClosed(t *testing.T) {
	t.Parallel()
	scoped := card(t, "Name:Scoped\nManaCost:1 U\nTypes:Instant\n"+
		"A:SP$ Draw | Defined$ You | NumCards$ 1\n"+
		"S:Mode$ AlternativeCost | ValidSA$ Spell.Samurai | Cost$ 0\nOracle:x\n")
	e := handEngine(t, scoped)
	scopedID := e.G.Zone(state.ZHand, 0)[0]
	if alts := altCastOptions(e.legalActions(0), scopedID); len(alts) != 0 {
		t.Fatalf("unevaluable ValidSA$ constraint still granted the free cast: %+v", alts)
	}
}
