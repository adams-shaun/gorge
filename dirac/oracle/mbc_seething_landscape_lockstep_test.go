package oracle

import "testing"

// Seething Landscape's "Cycling {U}{B}{R}" is payable from the hand with a
// seat's only coloured sources a Swamp and Llanowar Envoy's repeatable "{1}{G}:
// Add one mana of any color" (no {T}): the Envoy is activated twice off the
// Forests and floating mana, once for {U} and once for {R} (CR 602.2b, 605.3b,
// 601.2g). gorge's potential plan prices the cycling and offers it; the kernel's
// hand-activation census counted the repeatable filter once and left it out
// (decision 1035 of the recorded game, pool tail-28-c000 pair 9, autopay).
func TestMBCSeethingLandscapeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cycling_beside_repeatable_net_negative_filter")
}
