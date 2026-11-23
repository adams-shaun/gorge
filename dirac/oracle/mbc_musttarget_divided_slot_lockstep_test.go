package oracle

import "testing"

// TestMBCPyrokinesisReprosLockstep: the caster of Pyrokinesis ("deals 4 damage
// divided as you choose among any number of target creatures") is the opponent
// of a Standard Bearer's controller. The Flagbearer static obliges at least one
// Flagbearer in the answer, so the divided slot's Done is not offered while a
// Flagbearer is a legal target. The kernel once skipped the obligation for
// divided slots and offered done (dr-mbc-baaa pair 31, decision 498).
func TestMBCPyrokinesisReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "musttarget_divided_slot")
}
