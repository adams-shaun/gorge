package oracle

import "testing"

// Meteor Blast (X R R R, "4 damage to each of X targets") at a p1 priority
// decision under Blood Moon: gorge offered cast(Meteor Blast) where the kernel
// did not (decision 576, tail-23 pair 23).
func TestMBCMeteorBlastReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "meteor_blast_options_priority")
}
