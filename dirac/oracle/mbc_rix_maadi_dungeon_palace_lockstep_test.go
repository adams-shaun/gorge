package oracle

import "testing"

// CR 601.2g, 605.3b: Magus of the Coffers' "{2}, {T}: Add {B} for each Swamp you control" (a one-colour
// dynamic fee filter) poses no colour ask in gorge's prefix search, so an activation of it is no extra node.
// The kernel charged a node for the ask on every Magus step, spent the 160-node budget on the Vivid Creek
// {R} line before reaching it (168 nodes against gorge's 136) and left Rix, Dungeon Palace's
// "{1}{B}{R}, {T}" sorcery-speed activation unoffered (decision 653 of 653, dr tail-12 pair 6).
func TestMBCRixMaadiDungeonPalaceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "dyn_filter_fee_asks_no_colour_node")
}
