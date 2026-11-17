package oracle

import "testing"

// CR 106.6 with CR 118.12: a Vibranium token's {C} "can't be spent to cast a
// nonartifact spell" bars only casting, so it may pay Disrupt's "unless its
// controller pays {1}" beside tapped-out lands. gorge offers the pay mode;
// the kernel's unless gate counted no restricted source and offered decline
// only (decision 384 of tail-68 pair 164, the same shape as Thirst's
// "sacrifice unless you pay" modes decision in the tail-18 evidence deal).
func TestMBCThirstReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_cost_paid_with_artifact_only_restricted_mana")
}

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
