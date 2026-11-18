package oracle

import "testing"

// CR 702.49a, 614.12, 707.2: Sakashima's Student put onto the battlefield by its own
// ninjutsu is asked its "may enter as a copy" election, as gorge asks every battlefield
// entry (the permanent waits in hand while the ability resolves); the kernel entered it
// as itself without the ask (tail-11-c000 pair 21, decision 378).
func TestMBCSakashimasStudentReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ninjutsu_entry_asks_clone_election")
}
