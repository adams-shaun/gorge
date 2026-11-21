package oracle

import "testing"

// CR 118.12 with CR 605.3b: Conversion's upkeep "sacrifice unless you pay
// {W}{W}" is payable from Plains for {W} plus Unknown Shores' "{1},{T}: Add
// one mana of any color" funded by an Island, so gorge offers the pay mode
// beside the decline (tail-34 pair 40, seed 382028).
func TestMBCChosenByHeliodReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_cost_funded_by_any_colour_land")
}
