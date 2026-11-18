package oracle

import "testing"

// CR 119.4 and CR 605.3b with CR 702.27a: a player at 1 life holding Searing
// Touch beside City of Brass (its tap deals the tapper 1 damage) and four
// ordinary lands can cast the plain {R} Searing Touch but not the {R}{4}
// buyback cast, since paying the fifth mana from City of Brass ends them.
// gorge's planner never offers a plan whose summed pain reaches the payer's
// life, and the kernel's kicked-cast potential proof counted the City of
// Brass tap anyway, so it listed the buyback cast gorge withheld (decision 925
// of 925, pool tail-57 pair 190).
func TestMBCSearingTouchReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "kicked_cast_lethal_pain_source")
}
