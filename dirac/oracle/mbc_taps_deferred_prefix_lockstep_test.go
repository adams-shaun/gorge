package oracle

import "testing"

// CR 602.2b, 603.2: Scald makes every Island a Taps-deferred source, and each
// Island tapped for mana queues a trigger dealing 1 damage to its payer. At 2
// life Grasping Dunes' "{1}, {T}, Sacrifice: put a -1/-1 counter on target
// creature" (sorcery speed) is offered by gorge's prefix search with ONE
// Island tapped and its trigger resolved (1 life left); tapping all five
// Islands is lethal. The kernel's search asked for the play only after every
// gap was tapped, so it found no script and dropped the offer (decision 854;
// tail-49 pair 22).
func TestMBCGraspingDunesReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "taps_deferred_prefix_resolves_before_lethal_taps")
}
