package oracle

import "testing"

// Avarice Amulet's equip activation is offered by gorge at decision 604 of
// pool tail-17 pair 232 and not by dirac's priority offer. The recording
// replays on the custom table its own decks need: Shimmerdrift Vale has
// drifted out of tail-17, so the synthetic pool no longer holds the game.
func TestMBCAvariceAmuletReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "avarice_amulet_equip_offer")
}
