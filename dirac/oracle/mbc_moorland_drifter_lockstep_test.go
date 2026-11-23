package oracle

import "testing"

// CR 111.7 with CR 704.5d and CR 207.2c: a token that reached the graveyard (Arcum
// Dagsson sacrificing an artifact creature token) is no card until state-based actions
// remove it, so its types don't count toward Moorland Drifter's delirium. The kernel
// counted them and gave the Drifter flying (tail-53-c000 pair 42, decision 211).
func TestMBCMoorlandDrifterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "moorland_drifter_delirium_flying")
}
