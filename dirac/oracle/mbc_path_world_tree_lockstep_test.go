package oracle

import "testing"

// CR 601.2h, 605.3b: Path to the World Tree's {2}{W}{U}{B}{R}{G}, Sacrifice
// Path activation is payable from a Darkslick Shores, a Lotus Blossom holding a
// petal (one mana of one colour, tap and sacrifice), a Gift of Paradise land (two
// mana of one colour) and plain Plains and Forests, so gorge offers it at
// priority and the kernel must too (tail-74 pair 49, decision 610).
func TestMBCPathToTheWorldTreeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "path_world_tree_dynsac_grant_activation")
}
