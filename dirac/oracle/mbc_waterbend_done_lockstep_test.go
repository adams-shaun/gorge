package oracle

import (
	"testing"
)

// CR 701.67a with CR 601.2g-h: while paying a waterbend cost, each untapped
// artifact or creature tapped pays for {1} of it, and the window's sources pay
// the rest. A "done" closing the waterbend announcement at zero picks is legal
// only when the pool and the window can pay the whole cost without any tap:
// Ruthless Waterbender's Waterbend<2> pump with a window that cannot fund both
// generic must demand the first tap. The kernel's tapsNeeded probe once read
// zero taps as enough and posed done beside the picks (decision 492 of 492;
// tail-13-c000 pair 39).
func TestMBCRuthlessWaterbenderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "waterbend_done_zero_taps")
}
