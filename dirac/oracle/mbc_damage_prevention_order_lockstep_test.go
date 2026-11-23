package oracle

import "testing"

// CR 616.1: the player a damage event would hit orders the prevention effects
// that apply, and Battletide Alchemist's Optional$ line adds a skip option.
// Beside Commencement of Festivities' combat-only full prevention gorge asks
// that mixed order (tail-76 pair 2, decision 654); the kernel's combat walk
// now poses the same CR 616.1 order with the Optional$ skip (spCombatCands,
// CR 616.1g), so the recording replays lockstep to the end.
func TestMBCBattletideAlchemistReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "damage_prevention_order_alchemist_festivities")
}
