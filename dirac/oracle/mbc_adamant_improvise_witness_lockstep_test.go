package oracle

import (
	"testing"
)

// CR 702.126a with CR 601.2g-h: Rally for the Throne's Adamant rider reads the
// mana the cast spends, and Inspiring Statuary's "nonartifact spells you cast
// have improvise" grants the instant an optional credit beside it. The cast is
// payable at its printed cost from the lands alone, so the potential planner's
// printed-cost witness is a proof and the priority vocabulary must offer the
// cast; the kernel (dirac) offered it, gorge's planner declined it as an
// unknown credit shape and the menus parted (options options, decision 601).
func TestMBCRallyForTheThroneReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "adamant_improvise_witness")
}
