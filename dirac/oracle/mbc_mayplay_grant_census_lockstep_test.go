package oracle

import "testing"

// CR 118.9 with CR 601.2g: Elven Chorus lets its controller cast creature
// spells from the top of their library, and the top card is Grazing Whiptail
// ({2}{G}{G}) with every land tapped. The chorus's second static gives the
// creatures "{T}: Add one mana of any color", so the cast is payable and
// gorge's potential may-play arm prices it from its planner census, granted
// mana abilities included. grantCap repruned the built offer with the plain
// cast's ungranted census and dropped it when its prefix search ran out of
// gorge's node budget.
func TestMBCGrazingWhiptailReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mayplay_grant_census")
}
