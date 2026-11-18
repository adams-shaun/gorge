package oracle

import "testing"

// CR 603.7: a delayed triggered ability's source is the object that created
// it, so Sudden Disappearance's "return the exiled cards at the beginning of
// the next end step" trigger is sourced by the spell, not by each exiled card.
// The gorge side pushed the TrigReturn delayed trigger sourced by Sudden
// Disappearance while dirac sourced it by an exiled card (Vedalken Plotter):
// tail-13-c000 pair 4, seed 16282190910774938203, decision 445.
func TestMBCSuddenDisappearanceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trigreturn_delayed_source_not_the_spell")
}
