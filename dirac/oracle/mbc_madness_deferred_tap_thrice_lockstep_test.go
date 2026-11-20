package oracle

import (
	"testing"
)

// Revolutionist's madness cast-or-graveyard ask beside Burning Earth: the
// watcher (TapsForMana, TriggeredActivator, one literal damage) defers every
// nonbasic land's tap, so the cast's census omits the only two untapped mana
// sources -- Crumbling Necropolis ({T}: Add {U}, {B}, or {R}) and A-Faceless
// Haven ({T}: Add {C}) -- and the pool holds no floating mana. The kernel's
// potential-defer arm once priced the offer on the potential bound alone,
// whose per-ability over-bound folds the Necropolis's three produced faces as
// three independent units: four mana from two taps, so "yes" was offered for
// a {3}{R} madness cost gorge proves unpayable (altcast.go provenUnpayable:
// the relaxed census's and the count's legs -- the relaxed runs tap each
// deferred source once, two mana in all; CR 601.2g-h). The offer is bounded
// by those proofs (kw_madness.go madnessDeferProofClass).
func TestMBCMadnessOfferBoundedByProofsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "madness_cast_offer_counts_a_deferred_tap_thrice")
}
