package oracle

import "testing"

// CR 118.12a with CR 106.6: Geistlight Snare counters Ashen Powder unless its
// controller pays {3}. The controller has two untapped Islands and two
// Vibranium tokens ("{T}: Add {C}. This mana can't be spent to cast a
// nonartifact spell"), which bars a spell cast only, so a token's {C} pays the
// unless tax and the third mana makes "pay" a legal answer. The kernel priced
// the unless cost from the unrestricted sources alone and offered only the
// decline (decision 498 of 579, synth tail-70 pair 42).
func TestMBCGeistlightSnareReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_cost_pays_with_nonspell_restricted_mana")
}
