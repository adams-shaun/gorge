package oracle

import "testing"

// CR 118.3, 605.3b, 608.2g: Conversion's "sacrifice unless you pay {W}{W}"
// with Elminster's Simulacrum just paid for, leaving Island, Unknown Shores
// and Botanical Plaza untapped. The window pays it: the Island's mana funds
// Unknown Shores' "{1}, {T}: Add one mana of any color" for the second {W}
// beside the Plaza's. Gorge posed pay and decline; the kernel offered decline
// only, since its unless window priced a second-ability filter only on a
// table with a first-only filter (decision 432 of tail-34 pair 5).
func TestMBCElminsterSSimulacrumReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_pays_through_second_ability_filter")
}
