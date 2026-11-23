package oracle

import "testing"

const shatteringSpreeRepro = "shattering_spree_replicate_search_budget"

// CR 702.56a, 106.1: a replicated cast of Shattering Spree ({R} plus a
// replicate {R}) is paid here only through the gap sources: a Forest wearing
// Sheltered Aerie ("{T}: Add two mana of any one color") and a Chromatic Star
// ("{1}, {T}, Sacrifice: Add one mana of any color"). The recorded game
// (tail-62 pair 51, decision 273) once showed gorge withhold the cast the
// kernel offered. The Multikicker/replicate granted-mana prefix search now
// prices the base plus one payment (3537f62ce1), so both engines
// offer it; the card is no longer refused (dirac/support/unrefused.tsv) and
// this test pins the unrefusal's proof.
func TestMBCShatteringSpreeReprosLockstep(t *testing.T) {
	res := replayRawRepro(t, shatteringSpreeRepro)
	if res.Div != nil {
		t.Fatalf("%s's recorded game diverges at %d: %s", shatteringSpreeRepro, res.Div.At, res.Div)
	}
	if res.StallUnmodelled {
		t.Fatalf("%s's recorded game ends at an unmodelled stall: %q", shatteringSpreeRepro, res.Stall)
	}
}
