package oracle

import (
	"testing"
)

// CR 117.8e with CR 601.2g: a target-dependent cost raise prices a free
// cast's menu against the CR 601.2g window too. p1 casts Aether Tradewinds
// free off Urza, Lord High Artificer's may-play rider, and p0's Kasmina,
// Enigmatic Mentor taxes {2} every target that is a creature or planeswalker
// she controls. p1's pool is empty; the two units that reach the tax are
// Birds of Paradise's tap and Urza's own "tap an untapped artifact: Add {U}"
// -- a mana ability whose cost is no bare tap, so only the window census
// (not the plain pool census) lists it. The kernel once priced the free
// cast's taxed candidates against the plain avail alone and withheld
// Hammers of Moradin, Kasmina, Tolarian Terror and both wizard tokens from
// the menu (decision 696 of the pair; dr-mbc-baaa).
func TestMBCHammersOfMoradinReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "free_cast_tax_window")
}
