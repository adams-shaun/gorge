package oracle

import "testing"

// CR 616.1: the player a damage event would hit orders the prevention effects
// that apply, and Battletide Alchemist's Optional$ line adds a skip option.
// Beside Commencement of Festivities' combat-only full prevention gorge asks
// that mixed order (tail-76 pair 2, decision 654); the kernel's combat walk
// poses it (the optional line beside full preventions, spOptionalAsk), so the
// recording replays in lockstep with no stall.
func TestMBCBattletideAlchemistReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "damage_prevention_order_alchemist_festivities")
}
