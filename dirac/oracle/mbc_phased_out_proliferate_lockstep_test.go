package oracle

import "testing"

// CR 702.26b with CR 701.34a: a phased-out permanent is treated as though it
// does not exist, so proliferate cannot choose it even while it holds +1/+1
// counters. gorge's proliferate ask once listed a phased-out Goblin Cadets
// beside the visible permanents; the kernel never offered it (decision 846 of
// the tail-61-c000 evidence game, pair 199).
func TestMBCGoblinCadetsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phased_out_not_proliferate_recipient")
}
