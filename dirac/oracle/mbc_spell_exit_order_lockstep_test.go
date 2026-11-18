package oracle

import "testing"

// CR 616.1 with CR 608.2k: a resolved spell's move to the graveyard competes
// between two Rests in Peace, and the player who orders them is the spell's
// controller, not its owner. Rally the Monastery, cast by p1 from p0's
// Griffin Guide / Goblin Wizardry pick, is p0's card controlled by p1: gorge
// asks p1, and dirac asked the owner p0 (dr-mbc-aaba pair 114, decision 572).
func TestMBCHealerOfThePrideReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "spell_exit_replacement_order_asks_controller")
}
