package oracle

import "testing"

// CR 106.6, 305.5: a Powerstone token enchanted by Encrust keeps its printed
// restricted {C} as a window gap (windowGap lists it, no autopay source), but
// Encrust's CantBeActivated static locks it, the same gate gapMask applies to
// the restricted mask. The kernel walked the window without the lock and
// panicked "index out of range [-1]" at eachFee (tail-35 pair 253, decision
// 671); gorge's windowGap must read the lock and skip the source.
func TestMBCPowerstoneWindowLockedReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "powerstone_window_locked_by_encrust")
}
