package oracle

import "testing"

// A convoke cast whose planner witness no longer matches (the announced picks
// changed the cost) falls back to the manual window, whose colour ask carries
// no ManaPayment. Seat 1 casts Markov Baron {2}{B} convoking Arrogant Vampire
// and Citywatch Sphinx beside Chromatic Lantern: gorge lists the Lantern's
// any-colour ability on the first Island ahead of its {U} (the unreduced cost
// owes {B}), and the bot answers that ability's lowest colour, so {W} floats.
// The kernel's window taps the Island for its own {U} instead (CR 601.2g-h,
// CR 702.51a): windowGrantFirstCol (dirac/mana_grantfirst.go).
func TestConvokeLeftoverManaLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "convoke_leftover_mana_grant_first")
}
