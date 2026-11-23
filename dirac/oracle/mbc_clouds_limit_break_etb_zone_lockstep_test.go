package oracle

import "testing"

// CR 702.183a, 601.2g, 601.2h: Cloud's Limit Break's Blade Beam tier adds {1}
// to its printed {1}{W}. With one R and one W already floating and no tapped
// creature (so Cross-Slash has no target), the third mana comes from Mercadian
// Bazaar's storage counter in the cast's mana window; gorge pays it and puts
// the spell on the stack with no targets. dirac's post-announcement
// affordability gate counted no counter-cost source, so it reversed the cast
// (the card stayed in hand, the mana floated) (tail-26 pair 71, decision 122).
func TestMBCCloudsLimitBreakReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "tiered_blade_beam_storage_counter_beside_floating_mana")
}
