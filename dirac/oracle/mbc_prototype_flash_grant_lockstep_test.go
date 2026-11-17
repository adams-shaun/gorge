package oracle

import "testing"

// CR 718.3a: a prototyped spell has only the inset's mana cost, power,
// toughness and (CR 718.3a via the card's colour) colour while it is cast, so
// Yeva, Nature's Herald ("you may cast green creature spells as though they
// had flash") lets Cradle Clearcutter be cast prototyped ({2}{G}) in the
// opponent's upkeep. gorge judged the grant against the colourless printed
// face and never offered it (decision 362 of 362, tail-49-c002 pair 152).
func TestMBCCradleClearcutterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "prototype_cast_flash_grant_reads_the_inset_colour")
}
