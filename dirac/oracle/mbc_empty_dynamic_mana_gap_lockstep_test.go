package oracle

import (
	"testing"
)

// CR 601.2g: Baldur's Gate's second mana ability ("{2}, {T}: Add X mana of
// any one color, X = other Gates you control") makes nothing on this board,
// so gorge's planner window never taps it. The kernel's X-bound offer sweep
// (Killing Glare, {X}{B}, destroy a creature with power X or less) kept it as
// an open production whose {2} fee gapped the per-tap census, so the sweep
// priced X off the over-bound pool and offered a cast no announcement could
// pay (decision 69 of the recorded game: only gorge's menu lacks it). The
// cast offer itself is Killing Glare's separate repro, killing_glare_cast_offer.
func TestMBCEmptyDynamicManaGapReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "empty_dynamic_mana_gap")
}
