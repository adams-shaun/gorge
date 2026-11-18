package oracle

import "testing"

// CR 601.2c with CR 601.2f: Ghostfire Blade's Equip {3} costs {2} less when it
// targets a colorless creature, so the equip's target menu lists the creatures
// its controller can pay for. Jungle Delver and a token are payable at {3}
// through Ancient Tomb and a Forest, yet dirac listed only the colorless
// Aegis Automaton (decision 697 of tail-37-c000 pair 31).
func TestMBCJungleDelverReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ghostfire_equip_target_menu")
}
