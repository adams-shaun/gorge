package oracle

import "testing"

// The grant prefix search's listings counted a restricted mana ability whose
// mana gate is off. At decision 299 of pool tail-08 (pair 0, surface full)
// p0's sixth land (Island) brings The World Tree's six-lands grant live and
// Amnesia {3}{U}{U}{U} becomes payable from six lands; gorge offers
// cast(#0,-,0) (Amnesia). dirac's cast offer runs its grant prefix search on
// the cast, and the search's gapVisit/dual-flag candidates branch Shrine of
// the Forsaken Gods' restricted {C}{C} ability through ownRestrAb without
// checking its IsPresent$ "seven or more lands" gate — off at six lands —
// so it explored, with ghost-keyed states, a subtree gorge never offers
// (mana_gate.go: a gated-off ability is absent from the priority offer, the
// payment window and the planner census), exhausted its 160-node budget, and
// never reached the plan gorge's search finds; cast(Amnesia) was withheld
// (options options, lead gorge, implicates Amnesia).
func TestMBCAmnesiaReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "grant_search_lists_gated_off_restricted_mana")
}
