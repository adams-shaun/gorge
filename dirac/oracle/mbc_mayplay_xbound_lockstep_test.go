package oracle

import "testing"

// CR 601.2b, 601.2c, 601.2h: Killing Glare exiled by a Junk token is playable
// this turn, but its target (power X or less) needs X >= 2 against the only
// creature in play and one Treasure pays no more than X=0. A cast from exile
// is judged like a hand cast: some payable X must leave a legal target. gorge
// offered it (the may-play walk skipped xBoundTargetPayable); decision 445 of
// 445, evidence loop pool tail-51-c000 pair 3.
func TestMBCKillingGlareReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "may_play_x_bound_target_unpayable")
}
