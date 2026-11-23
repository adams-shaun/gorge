package oracle

import "testing"

// Wild Hunger's flashback {3}{R} on a board whose tap choices fit gorge's
// search budget: gorge's plain search funds the cast at node 150, the kernel's
// grantSearch mirror charged a second node for every filter colour ask the
// flattened "choose a mana ability" ask already covered and ran out at 160
// (tail-15, pair 99, decision 470: gorge offers, the kernel withholds).
func TestMBCGrantAskNodeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "wild_hunger_flashback_search_walk_askcharge")
}
