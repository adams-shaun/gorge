package oracle

import "testing"

// CR 605.3a with CR 601.2g-h: p1's Untaidake, the Cloud Keeper ("{T}, Pay 2
// life: Add {C}{C}. Spend this mana only to cast legendary spells") sat
// beside the fee-carrying filter Shadowblood Ridge ("{1}, {T}: Add {B}{R}"),
// and the plain cast of Sidewinder Naga ({2}{G}, funded by two Mountains and
// two Forests without either) still had to be offered. Registered as a census
// gap (gfGapOther) the restricted life source kept the filter gap walk off,
// and with gfGapOther absent from the walk's mask chain scriptOffers offered
// nothing at all: the kernel's priority list was [pass] where gorge's also
// listed the cast (decision 256 of the recording; tail-73 pair 1 before the
// restricted-life and filter-walk fixes). Both halves are load-bearing:
// reverting only one leaves the other papering over this game.
func TestMBCSidewinderNagaReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "restr_life_gap_plain_cast")
}
