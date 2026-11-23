package oracle

import "testing"

// CR 702.51a with CR 601.2g-h: a Worldsoul Colossus ({X}{G}{W}, Convoke) cast
// beside a tapped Combo Any source (Sage of the Maze) and untapped Plains
// convoke-taps its creatures, then reverses with "cast aborted: cost no longer
// payable" (decision 843 of tail-36 pair 66: gorge's payment walk finds no plan
// for the remainder after the convoke credits, while the kernel taps a Plains
// and completes the cast).
func TestConvokeComboSourceCastReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "convoke_combo_source_cast_aborts")
}
