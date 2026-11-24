package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestManaRiderSelfGainLifeLockstepMutation: Pristine Talisman's "you gain 1
// life" rider is what keeps the kernel in step with gorge. Remove the rider
// and the run must diverge. The mutation is a no-op unless the table's
// Talisman ability carries the self-life rider, so the test asserts it saw
// one, and that the deck cast the Talisman at all.
func TestManaRiderSelfGainLifeLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	sawRider := false
	mutate := func(tbl *kop.Table) {
		mutateCard(t, tbl, "Pristine Talisman", func(a *kop.Ability, in *kop.Instr) {
			if a.ManaRider == kop.ManaRiderSelfGainLife && a.ManaRiderN == 1 {
				sawRider = true
			}
			a.ManaRider = kop.ManaRiderNone
		})
	}
	// The mutation reads Pristine Talisman, so it plays the two matchups whose
	// decks carry it (the Vulshok Factory / Land Cap decks do not).
	run := runFixtureLockstep(t, "manarider", manaRiderMatchups[:2], manaRiderWatch, 120, false, mutate)
	if !sawRider {
		t.Fatal("Pristine Talisman's mana ability carries no self-life rider: the mutation is a no-op")
	}
	if run.casts["Pristine Talisman"] == 0 {
		t.Fatal("no game cast Pristine Talisman")
	}
	run.requireDivergence(t, "Pristine Talisman loses its life rider")
}
