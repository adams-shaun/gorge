package oracle

import (
	"testing"
)

// CR 106.6, 601.2g-h, 605.3a-b: Shadowblood Ridge's "{1}, {T}: Add {B}{R}"
// funds Malevolent Awakening's "{1}{B}{B}, Sacrifice a creature: Return target
// creature card from your graveyard to your hand" beside an untapped A-Base
// Camp (its {C} pays the Ridge's fee), an untapped Swamp and the graveyard's
// Primoc Escapee, with Untaidake, the Cloud Keeper (a restricted {T}, Pay 2
// life planner unit the payment admits nothing of) on the battlefield. gorge's
// planner prices the chain: the fee gap, not the restricted unit, is the board
// only census gap beside the filter. The kernel's filter search is
// filter-only (filtOnly), and Untaidake's PayLife cost flag registered it as
// a census gap (gapOthers), which withheld the search wholesale: the kernel
// offered pass only (decision 464 of pool tail-73, seed 848479, surface
// autopay). A restricted planner unit is no gap to the search.
func TestMBCMalevolentAwakeningReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "restrsrc_gapother_fee_chain_unoffered")
}
