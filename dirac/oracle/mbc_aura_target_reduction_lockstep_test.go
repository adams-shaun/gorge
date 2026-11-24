package oracle

import "testing"

// CR 601.2f with CR 601.2c: Strong Back makes Aura spells that target its
// enchanted creature cost {3} less, so Favor of the Overbeing ({1}{G/U}) aimed
// at the Mystic wearing it costs {G/U}, which the one floating {U} pays. Both
// engines offered the cast, then dirac's cast-time witness priced the spell
// before its target and panicked "autopay found no plan" (decision 155 of
// 155, tail-07-c000 pair 7).
func TestMBCFavorOfTheOverbeingReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "aura_cost_cut_by_its_target_witness_pays_reduced_cost")
}
