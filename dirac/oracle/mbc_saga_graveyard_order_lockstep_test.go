package oracle

import "testing"

// Inventor's Axe's lockstep class (projection Cards[i].Pos): a Saga that
// sacrifices a Mountain on its chapter and the Aura going to the graveyard
// beside it must land in the same graveyard order on both engines.
func TestMBCInventorSAxeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "saga_final_chapter_graveyard_order")
}
