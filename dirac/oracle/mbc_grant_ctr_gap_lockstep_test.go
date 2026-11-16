package oracle

import "testing"

// CR 605.1a, 601.2g-h: Wrenn and Realmbreaker grants every land an any-colour
// mana ability, and Wall of Roots' "put a -0/-1 counter on it: add {G}" is a
// census gap beside it. Four untapped lands and the Wall pay Iona's Judgment's
// {4}{W} in gorge's one prefix search over every gap. The kernel's grant
// search holds no counter-cost gap, gave the cast up as unpriced and left it
// to no other search (decision 553 of 553, pair 90 of tail-19).
func TestMBCIonaSJudgmentReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "grant_search_counter_cost_gap")
}
