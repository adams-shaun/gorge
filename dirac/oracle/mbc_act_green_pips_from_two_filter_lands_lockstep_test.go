package oracle

import "testing"

// CR 601.2g, 602.2b, 605.3a: Kamahl, Fist of Krosa's {2}{G}{G}{G} pump is payable from
// Turntimber Grove's {G} and two {1},{T} any-colour lands (Hall of Oracles, Shimmering Grotto),
// each fed by a Plains, plus two more Plains for the generic. Gorge offered the activation
// (decision 1312 of tail-56-c000 pair 5); the kernel's filter search left it out.
func TestMBCKamahlFistOfKrosaReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "act_green_pips_from_two_filter_lands")
}
