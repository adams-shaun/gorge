package oracle

import "testing"

// CR 118.9d with CR 702.120a: a spell cast without paying its mana cost still
// pays its additional costs, and Escalate is one, due for each mode chosen
// beyond the first. Boarding Party's cascade free-casts Borrowed Malevolence
// (Escalate {2}) after Boarding Party tapped the caster's last six lands, so
// only one mode is affordable. The gorge side once captured the Escalate
// parameter only on the hand-cast path, so the cascade's free cast offered a
// second mode where the kernel went straight to the target (decision 77 of
// custom pair, seed 8: decision kind ref modes p0, kernel target p0).
func TestMBCBoardingPartyReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "boarding_party_cascade_escalate_unaffordable")
}
