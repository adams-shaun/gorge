package oracle

import "testing"

// CR 106.6: Karn's upkeep {C} "can't be spent to cast nonartifact spells", so
// the floating mana pays a non-spell cost. Go-Shintai of Lost Wisdom's "you may
// pay {1}" at the end step is one: gorge spends the floating restricted {C}
// (mana_add n=-1) and taps nothing, where the kernel tapped a Forest for the
// cost and left both {C} floating (decision 420, pair 5 of pool tail-62,
// surface autopay).
func TestMBCGoShintaiReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trigger_cost_spends_floating_restricted_mana")
}
