package oracle

import "testing"

// CR 305.7 with CR 603.3b: Contaminated Ground leaves Unknown Shores only the
// Swamp's {B} (its filter ability is gone), so the planner defers it as a
// Taps-deferred gap and gorge taps it in its own decision before Viral Drake's
// activation; its "becomes tapped" trigger is queued and stacked below the
// activation (decision 905 of tail-34 pair 100). The kernel kept the printed
// card's filter flag, left the land out of the gaps and tapped it after the
// push, stacking the trigger above the ability.
func TestMBCViralDrakeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mana_ability_trigger_before_activation")
}
