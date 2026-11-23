package oracle

import "testing"

// CR 608.2h: "if you controlled that artifact" reads the destroyed artifact's
// last known controller. Gleeful Demolition destroys a Thraben Gargoyle its
// caster stole from the opponent's library, so it goes to the opponent's
// graveyard but the caster still gets the three Goblin tokens.
func TestMBCGleefulDemolitionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "destroy_remember_lki_controller_after_move")
}
