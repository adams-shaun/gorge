package oracle

import "testing"

// CR 106.4, 605.3b: Lotus Blossom's mana ability taps and sacrifices it to add
// one mana of any colour for each petal counter on it. The kernel priced the
// source as no unit at all (dynamic amount plus a sacrifice cost), so the plain
// walk never afforded an activation funded only by its petal mana. The menu
// below the recorded game is a {3}{W} activation dirac could not pay with the
// untapped Lotus Blossom beside Blighted Steppe (decision 432, tail-74 pair 17).
func TestMBCLotusBlossomDynamicManaLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mbc_lotus_blossom_dynamic_mana")
}
