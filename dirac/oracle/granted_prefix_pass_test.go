package oracle

import "testing"

// gorge's prefix search lets a trigger a step caused resolve (a pass, then the
// trigger's own ask) once the node's activations are spent, and counts those
// nodes against its clone budget: Gran-Gran's "whenever this becomes tapped, draw a
// card, then discard a card" under Great Divide Guide costs a pass and one node per
// card in hand at every leaf. The kernel's search follows, so Earthen Ally's
// five-colour activation is priced in the same 106 nodes on both sides (CR 601.2g,
// 605.3a) instead of the kernel finding it in 34 and offering a play gorge could not.
func TestGrantedPrefixTapTriggerPassLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "granted_prefix_tap_trigger_pass", "std-tla", Config{})
}

// A sorcery-speed play is not offered while a trigger its prefix caused is on the
// stack (CR 307.1), so gorge's search passes for the trigger to resolve before it
// prices the play, and the kernel resolves Gran-Gran's loot before the cast of Invasion
// Submersible too, instead of casting over it.
func TestGrantedPrefixSorceryWaitsForTriggerLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "granted_prefix_sorcery_waits_for_trigger", "std-tla", Config{})
}
