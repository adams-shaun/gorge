package oracle

import "testing"

// Reinforcements puts up to three target creature cards from a graveyard on
// top of their owner's library at once. gorge appends them at the library's
// bottom in move order and then asks the owner to arrange them (CR 401.4);
// the kernel placed each target on top as it moved, so at the same ask its
// group was in reverse move order. The projection diverged before the owner
// answered (synth tail-55 focus-Reinforcements pair 3).
func TestMBCMultiTargetLibraryTopArrangementLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "multi_target_library_top_arrangement")
}
