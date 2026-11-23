package oracle

import "testing"

// CR 601.2b: gorge's scriptNeedColours reads no pips for a flashback cast (it
// leaves a moded cast's base unread), so its prefix search walks the colour
// asks in engine order and Resonating Lute's "two mana of any one color" lands
// float W for Tome Blast's {4}{R} flashback beside the Mountain's R. The
// kernel's graveyard-cast hint (playNeed's graveyardCastNeedCost) must only
// steer a search gorge's order cannot finish: it floated R for every Lute tap
// (decision 246 of a std-sos game, Pool[1] 4W+R against 5R).
func TestFlashbackCastPoolColoursLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "flashback_cast_pool_colours")
}
