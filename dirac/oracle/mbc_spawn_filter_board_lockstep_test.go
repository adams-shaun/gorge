package oracle

import "testing"

// CR 601.2h / 602.2b, with pay.PlanLastResortCostOK: Golgari Cluestone's
// {B}{G},{T}, sacrifice ability is affordable only through Orochi Leafcaller
// ({G}: add one mana of any color) turning a Forest's mana into the {B}, since
// the Cluestone itself is tapped for the cost. An Eldrazi Spawn on the same
// board ("Sacrifice this: add {C}") is a last-resort tier of the planner, no
// census gap, so the board's only gap is the filter and gorge's prefix search
// finds the script. dirac counted the Spawn's sacrifice-only ability as a gap
// beside the filter, ran no filter search and offered no activation (tail-57
// pair 138, decision 442).
func TestMBCGolgariCluestoneReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "spawn_beside_fee_only_filter_is_a_filter_board")
}
