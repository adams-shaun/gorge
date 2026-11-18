package oracle

import "testing"

// CR 601.2b with CR 702.66a and 605.3b: Logic Knot ({X}{U}{U}, delve) is cast
// with {U}{U} floating, an untapped Izzet Signet (its {1} fee makes {U}{R}) and
// a three-card graveyard. X = 4 is payable (three delved, the Signet's net mana
// for the fourth), so gorge's X ask offers 0..4; the kernel priced the delve
// credit against the bare pool and stopped at 3 (decision 483 of tail-47-c000
// pair 53). The recording then answers the delve ask with no exile, which
// leaves X = 4 short: gorge's target menu withholds every candidate and
// reverses the cast, and the kernel now does the same instead of asking the
// target (decision 485).
func TestMBCLogicKnotReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "delve_x_ask_ceiling")
}
