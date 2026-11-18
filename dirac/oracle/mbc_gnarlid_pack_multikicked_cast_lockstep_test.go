package oracle

import "testing"

// CR 702.43: Gnarlid Pack's multikicked cast ({1}{G}, Multikicker {1}{G}) is
// the card's kicked cast plus the count ask that follows it, so the kernel
// offers it wherever gorge's potential planner prices the base plus ONE
// payment (tail-24-c002 pair 167, decision 343: gorge offered
// cast(#15{Gnarlid Pack},-,1) and the kernel listed only pass).
func TestMBCGnarlidPackReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "gnarlid_pack_multikicked_cast")
}
