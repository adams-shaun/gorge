package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestStdFinTieredModeCostWindowRestrictedMana: CR 601.2g, 106.6. Fire Magic's
// Firaga mode adds {5} to its printed {R}, and its cast pays through the CR
// 601.2g window because gorge's planner declines a modal-cost face. The
// window taps The Emperor of Palamecia, whose {U} floats restricted beside
// the pool; the payment must spend that admitted batch, not fall short of
// the generic it covers.
func TestStdFinTieredModeCostWindowRestrictedMana(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-fin")
	replayRepro(t, reg, set, "tiered_mode_cost_window_restricted_mana", true, false)
}
