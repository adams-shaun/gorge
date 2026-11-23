package oracle

import "testing"

// Gnottvold Slumbermound's second ability ({3}{R}{G}{G}, {T}, Sacrifice:
// destroy target land) was offered by gorge at a priority decision where the
// kernel left it out (decision 1367 of tail-55-c000 pair 45, autopay, seed
// 595286). The kernel's reflected-mana census for the activation drops its own
// tap-and-sacrifice source, so it cannot see the {R} the untapped Incubation
// Druid could produce by reflecting Gnottvold's own {T}: Add {R}.
func TestMBCGnottvoldSlumbermoundReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "gnottvold_slumbermound_destroy_offer")
}
