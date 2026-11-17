package oracle

import "testing"

// CR 605.1a, 601.2g: Razortide Bridge ({T}: Add {W} or {U}) under the
// opponent's Pooling Venom ("whenever enchanted land becomes tapped, its
// controller loses 2 life") is a source gorge's planner defers: its census
// never lists it, and the cast it funds is reached through the prefix search,
// which taps it for one colour of its production. The kernel modelled that gap
// only for an any-colour or one-colour production, so the two-colour Bridge
// kept its census unit and the kernel offered Throne Warden ({1}{W}, with a
// Mountain) where gorge offered only pass (decision 399; tail-49 pair 175 seed 6).
// The recording plays on the table its own decks need (pool custom).
func TestMBCThroneWardenReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "taps_deferred_combo_source_is_a_gap")
}
