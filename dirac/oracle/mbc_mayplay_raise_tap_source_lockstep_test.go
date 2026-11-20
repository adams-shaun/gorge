package oracle

import "testing"

// CR 601.2f/118.9d with CR 601.2g: a noncreature spell cast without paying
// its mana cost still owes Thorn of Amethyst's {1}, and the caster activates
// the mana abilities the plan names. Urza, Lord High Artificer's exile grant
// holds Battle of Wits at the end of tail-64 pair 75; the raise is paid by
// the Mountain the seat had just played, the planner's witness tie-break
// (pay.CompareSteps, the source's object ID), not the first Mountain in
// battlefield order.
func TestMBCMayplayRaiseTapSourceLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mbc_mayplay_raise_tap_source")
}
