package oracle

import (
	"testing"
)

// Rude Awakening's entwined cast beside Diamond Kaleidoscope's printed
// "Sacrifice a Prism token: Add one mana of any color" (pair 39 of the
// tail-12 evidence pool): gorge's payment census marks the Prism-sac mana
// ability an unpriceable gap (a non-tap-only cost is a deferred shape, and
// its relaxation has no sound once-only bound), so the entwined cast's
// verdict is census_incomplete and its prefix search cannot fund three
// any-colour activations within its node budget — gorge drops the offer.
// The kernel's potential census priced the same repeats as payable units
// (sacAnyRepeatUnits) and offered the kicked cast, diverging at the
// priority menu.
func TestMBCRudeAwakeningReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "kicked_sac_repeat_gap")
}
