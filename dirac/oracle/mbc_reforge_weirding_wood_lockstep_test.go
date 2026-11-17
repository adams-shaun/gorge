package oracle

import "testing"

// CR 601.2g, 605.3b: Reforge the Soul's miracle cast ({1}{R}) pays through the
// manual mana window. gorge's window lists every untapped source in battlefield
// order and takes the first, so it taps Forest for {G}, then a Forest enchanted
// by Weirding Wood ("{T}: Add two mana of any one color") for {R}{R}, pays the
// cost with the two red and leaves the {G} floating. dirac's window left no
// green in the pool (projection Pool[0][4]; kfuzz tail-20 pair 43 of the focus
// list Reforge the Soul + Weirding Wood, full surface, mulligans 1, seed 3; first
// filed as tail-20 pair 15 seed 1). Played on its own decks' table: tail pool
// membership moves with the corpus.
func TestMBCReforgeTheSoulWeirdingWoodReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "miracle_cast_window_weirding_wood_pool")
}
