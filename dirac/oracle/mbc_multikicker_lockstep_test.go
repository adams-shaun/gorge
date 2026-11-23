package oracle

import "testing"

// CR 702.43 with CR 603.10a: Lightkeeper of Emeria's enters trigger gains 2
// life for each time it was kicked, and it is still on the stack when
// Mondrak, Glory Dominus's activation sacrifices the Lightkeeper as a cost.
// gorge reads the count off the source's last known information (2 life);
// the kernel read 0 once the source had left the battlefield (decision 643 of
// 643: tail-21-c000 pair 2, life 8 vs 6).
func TestMBCCandlelitCavalryReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "multikicker_count_survives_sacrificed_source")
}
