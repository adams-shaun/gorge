package oracle

import "testing"

// CR 205.3i and 305.7 with CR 611.2c: Awakener Druid animates a land that
// Lush Growth already made a Mountain, Forest and Plains. The land keeps those
// three land types and gains Treefolk, so its subtype set agrees on both
// engines (decision 638 of a tail-67 game). The recording's pool is custom: the
// synthetic tail-67 deal shifted when the card table moved, and no current
// synthetic pool holds the game's cards, so the repro rebuilds its table from
// its own decks (reproSet's documented custom path). The game is unchanged.
func TestMBCTail67ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "treefolk_animate_beside_land_types")
}
