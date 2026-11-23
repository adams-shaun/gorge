package oracle

import "testing"

// Alchemist's Talent at level 2 turns each of its controller's Treasures into
// a two-mana source; with three Islands and Graven Cairns also untapped the
// controller can pay Consecrated Sphinx ({4}{U}{U}) and Jokulhaups ({4}{R}{R}).
// One engine offered both casts and the other neither (decision 371 of the
// tail-16-c000 evidence pair 126).
func TestMBCConsecratedSphinxReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "consecrated_sphinx_treasure_class_cast")
}
