package oracle

import "testing"

// CR 605.3a, 601.2g-h: Goblin Battle Jester ({3}{R}) is castable from a board
// whose only red source is Heap Gate's "{1}, {T}: Add one mana of any color"
// filter, paid from another land, with Remote Farm's depletion-counter {W}{W}
// ability beside it. gorge's priority list offered the cast; the kernel's did
// not (decision 301 of tail-51 pair 4, options options). The recording is pool
// "custom" (a table built from its own decks), as the Persist one is.
func TestMBCGoblinBattleJesterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_beside_depletion_land_funds_cast")
}
