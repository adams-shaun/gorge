package oracle

import (
	"testing"
)

// Gnottvold Slumbermound's "{3}{R}{G}{G}, {T}, Sacrifice: Destroy target land"
// is payable at p1's priority (decision 693 of tail-55 pair 195, surface
// autopay) with no red land on the board: Incubation Druid's reflected
// Amount$ mana ability ("Add one mana of any type that a land you control
// could produce") is a census gap, and gorge's prefix search resolves it for
// real (Gnottvold itself is a land that could produce {R}), so gorge offers
// the activation. The kernel's census gap read counts the reflection as {C}
// only, and the gaps walk used to take the activating land's own {T} source
// out of the census, which left the destroy short its own {R} and unoffered.
// Beside a Combo Any / reflected Amount$ gap the affordability check leaves
// the land's own unit in the census (mana_reflamtowntap.go ownTapBoundKept,
// CR 602.2b), so the destroy is the gaps walk's play and reaches caSearch:
// with that keep neutered this repro fails with the original divergence
// (`divergence at 693 of 693: options options []`).
func TestMBCGnottvoldSlumbermoundReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "gnottvold_reflamt")
}
