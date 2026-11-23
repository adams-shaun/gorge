package oracle

import (
	"testing"
)

// Shadowblood Ridge's "{1}, {T}: Add {B}{R}" beside an untapped Island and
// Mountain, a Cinder Barrens and Galazeth Prismari's Treasure token, holding
// Diver Skaab ({3}{U}{U}): both engines must offer the cast. It is legal per
// CR 601.2g -- tap Cinder Barrens for {B}, pay the Ridge's {1}, and the
// Ridge adds {B}{R}; the Island and the Treasure pay the two {U}. Gorge's
// prefix search used to offer pass only: it tapped the Treasure before the
// Ridge, and the Ridge's generic fee then spent the Treasure's {U} (the
// generic deduction's WUBRG order), stranding a {U} the cast still needed.
// The fee-order fallback funds first, then pays the fee gap, then taps the
// free any-colour gap. The kernel offered the cast (decision 439 of pool
// tail-73, surface full, seed 281703); this repro records the gorge fix.
func TestMBCDiverSkaabReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cast_offer_counts_mana_fee_source")
}
