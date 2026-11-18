package oracle

import "testing"

// CR 702.127a, 709.3: Road // Ruin in its owner's graveyard offers Ruin only
// when a creature Ruin (red) can target exists. Akroma's Memorial gave the
// board's only creature protection from red, yet gorge judged the targets with
// the green front showing and offered the cast; the kernel was right (tail-29-c001
// pair 7, decision 384).
func TestMBCRoadReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "aftermath_targets_judged_as_the_half")
}
