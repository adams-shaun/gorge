package oracle

import "testing"

// CR 106.6 with CR 118.12: Karn, Legacy Reforged floats {C} that can't be
// spent on nonartifact spells, and the unless tax Kataki, War's Wage grants
// ("sacrifice this artifact unless you pay {1}") is no spell, so the floating
// mana pays it. Gorge offered pay and decline on the second artifact's tax
// with only that mana left; the kernel priced the tax against plain pool and
// source mana alone and offered decline only (decision 541 of dr-mbc-abab
// pair 121).
func TestMBCSkeletalChangelingReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_cost_spends_restricted_pool_mana")
}
