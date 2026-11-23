package oracle

import "testing"

// CR 601.2b with CR 702.51a: Worldsoul Colossus's {X} is announced while a
// Faithful Watchdog is already committed to Convoke for its generic half, and
// the only mana left is Sage of the Maze's "add two mana in any combination of
// colors". Gorge's X menu priced that source as two mana of ONE colour, found
// no payable X, and fell back to X = 0 -- an X the announced generic tap cannot
// absorb. The window must price the source's split, so X = 0 is rejected and
// X = 1 offered (decision 260, tail-36 pair 2 seed 307878).
func TestMBCFaithfulWatchdogReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cast_window_combo_any_x_after_convoke_pick")
}
