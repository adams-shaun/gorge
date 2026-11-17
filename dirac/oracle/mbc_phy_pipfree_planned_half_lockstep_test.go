package oracle

import "testing"

// CR 601.2b: a spell's Phyrexian pips are announced (each face — its colour
// or two life — chosen) before the CR 601.2g-h payment, so gorge's cast
// planner prices the pip-free mana half alone (pay.PlanManaHalf): the
// witness funds {2} from two Plains and the pip ask answers the colour face
// from the tapped mana, the manual window's taps paying the rest. The
// kernel's offer-gate census priced the pip as a mana pip of the full cost
// ({2}{W/P} = 3 units from a 2-unit census at 1 life, the pain land's colour
// behind the planner's lethal guard), withholding a plain cast gorge's
// planner prices (pair 141 of pool dr-mbc-baab, decision 432;
// Porcelain Legionnaire). The exact name the brief asked for,
// TestMBCPorcelainLegionnaireReprosLockstep, is taken by
// mbc_phyrexian_pip_deferred_lockstep_test.go (a different pip cause).
func TestMBCPhyPipFreePlannedHalfReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phy_pipfree_planned_half")
}
