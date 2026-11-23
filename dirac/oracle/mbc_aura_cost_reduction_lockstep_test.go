package oracle

import "testing"

// CR 601.2f: Strong Back makes Aura spells its controller casts that target the
// enchanted creature cost {3} less, so with the Vampire token enchanted by it
// the controller can afford Sage's Reverie ({3}{W}) from three lands (decision
// 428 of tail-07-c000 pair 3, seed 612177). gorge offered the cast; dirac's menu
// withheld it.
func TestMBCSageSReverieReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "aura_cost_reduced_by_enchanted_creature")
	// The same game 44 decisions on: casting a second Strong Back. Its own
	// battlefield "Aura spells that target enchanted creature" reduction prices
	// other spells, not this cast's, so the target menu must not filter the
	// candidates through it (the kernel reversed the cast with none left, where
	// gorge pushed it and asked for a target).
	stdReproNamed(t, "aura_cast_own_battlefield_reduction_not_own_price")
}
