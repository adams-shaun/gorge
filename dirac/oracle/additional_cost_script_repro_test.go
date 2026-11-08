package oracle

import "testing"

// TestStdEclAdditionalCostScriptReprosLockstep: std-ecl's additional-cost
// casts whose composed mana half cannot be planned from the census alone. The
// repro is the divergence at decision 836 of pair 92: Champions of the Perfect
// ({3}{G}, "as an additional cost to cast this spell, behold an Elf and exile
// it") is a legal potential play whose fourth mana comes from Springleaf Drum,
// a source the planner's census cannot price, so the panel must still reach
// the census/prefix search and tap it (CR 601.2b, 601.2g).
func TestStdEclAdditionalCostScriptReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-ecl", []stdRepro{
		{"additional_cost_insufficient_reaches_prefix_search", false, false},
	})
}
