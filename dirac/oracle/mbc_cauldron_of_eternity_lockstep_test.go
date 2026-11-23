package oracle

import "testing"

// The Cauldron of Eternity's {2}{B}, {T}, pay 2 life ability targets a creature
// card in its controller's graveyard; gorge and dirac must offer it at the same
// priority decisions (tail-71 pair 29, decision 1077).
func TestMBCTheCauldronOfEternityReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cauldron_of_eternity_graveyard_target")
}
