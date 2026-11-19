package oracle

import "testing"

// The Mana Rig's "{X}{X}{X}, {T}: look at the top X cards" X ask again, with
// Sunscorched Divide's wide {1} fee and Fetid Heath on the board (tail-01
// pair 612, seed 525781, decision 458). The kernel's filter walk kept
// Heath's own {C} standing as the fees' mana while Heath's {W/B} ability
// still made its two (CR 605.1a: the permanent's one tap made it), so it
// priced 9 units off a window that reaches 8 and offered one X past the
// largest payable, X = 2 (CR 601.2b, 601.2g). The same fix's hybrid-fee half
// is TestMBCTheManaRigHybridFeeReprosLockstep (pair 122's gorge probe).
func TestMBCManaRigReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mana_rig_filter_own_tap_x_bound")
}
