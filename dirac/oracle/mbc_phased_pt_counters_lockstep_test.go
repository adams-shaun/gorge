package oracle

import "testing"

// CR 702.26b, 122.1a: a phased-out creature is treated as though it doesn't
// exist for continuous effects, but its counters stay on it, so Wall of
// Roots's -0/-1 counter still lowers the toughness gorge projects while
// Guardian of Faith has it phased out (decision 649 of dr-mbc-bbaa pair 52).
// The kernel's phased P/T read only +1/+1 and -1/-1 counters and showed 0/5.
func TestMBCWallOfRootsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phased_out_creature_keeps_pt_counter_kinds")
}
