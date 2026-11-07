package oracle

import "testing"

// TestStdEclTapXScriptGapReprosLockstep: std-ecl's tapX-cost-beside-an-
// each-colour-source games. The repro is the divergence at decision 124 of
// pair 61: Wanderbrine Trapper's "{1}, {T}, Tap another untapped creature you
// control" was offered while Bloom Tender was the only other creature and its
// scripted each-colour mana was the only {1} (CR 601.2g-h).
func TestStdEclTapXScriptGapReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-ecl", []stdRepro{
		{"tapx_cost_beside_each_colour_source", true, false},
	})
}
