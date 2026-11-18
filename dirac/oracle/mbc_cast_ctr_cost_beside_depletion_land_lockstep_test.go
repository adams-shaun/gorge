package oracle

import (
	"testing"
)

// Noose Constrictor ({1}{G}) in hand beside an untapped Forest and a Treasonous
// Ogre ("Pay 3 life: Add {R}"): the Forest pays the {G} pip and the Ogre's life
// unit pays the generic (CR 601.2g: a mana ability with a non-tap cost is
// activatable in the cast window), so gorge's planner, which prices the cast
// over its relaxed census, offers the cast in its controller's main phase
// (decision 346 of pair 97, pool dr-mbc-bbbb, surface autopay); the kernel's
// plain offer census held the printed tap sources only and offered pass and the
// land drop. The board also holds Peat Bog, a Deplete land the planner census
// leaves out.
func TestMBCNooseConstrictorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cast_ctr_cost_beside_depletion_land")
}
