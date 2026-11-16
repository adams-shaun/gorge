package oracle

import "testing"

// CR 708.5 hides a face-down permanent's NAME from seats that don't control
// it. A ChooseType Basic Land ask (Mystic Compass) ranges over the five land
// TYPE words -- a fixed vocabulary whose "Swamp" is the type, not the
// face-down card's name -- so the ask must offer all five whatever a
// face-down Swamp sits on the opponent's battlefield. Gorge once rewrote the
// type option's label to "a face-down permanent", dropping Swamp from its
// menu while the kernel kept all five: the option lists parted (decision 429
// of the recorded game, p0 choosing after Mystic Compass's activation
// resolved; a manifested Swamp#75 sat on p1's battlefield).
func TestMBCMysticCompassReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "facedown_type_label")
}
