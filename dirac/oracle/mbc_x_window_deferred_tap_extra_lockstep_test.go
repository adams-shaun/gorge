package oracle

import "testing"

// CR 601.2b, 107.3: an X cast's affordable X range counts every mana the
// controller's sources can add in the window. Orcish Settlers ({X}{X}{G},
// "Destroy X target lands") was offered by gorge but withheld by the kernel
// when a planner-deferred source's reflected tap extra went unpriced in the
// X window (decision 140, tail-73-c000 seed 352851). Fixed by e8a28dcd7f.
func TestMBCOrcishSettlersReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "x_window_deferred_source_tap_extra")
}
