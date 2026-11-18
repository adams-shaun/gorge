package oracle

import "testing"

// CR 601.2b/601.2g-h: Reshape ({X}{U}{U}, sacrifice an artifact as an additional
// cost) beside a Treasure that is the only artifact: the Treasure's mana needs
// the sacrifice the additional cost already spent, so it is no source for the
// cast. Pool tail-03-c001 pair 128 (p1: one untapped Island and the Treasure).
//   - sac_cost_treasure_also_mana_source: dirac offered the cast at {U}{U}
//     (decision 228) where gorge reserves the Treasure and offers none.
//   - sac_cost_treasure_x_window: with the Treasure held back, the largest-X
//     witness floats nothing more (decision 230); an announced X = 1 then pays
//     through the window first, takes the Treasure, and the sacrifice has no
//     candidate left, so the cast reverses with the mana floating (decision 231).
func TestMBCReshapeReprosLockstep(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"sac_cost_treasure_also_mana_source", "sac_cost_treasure_x_window"} {
		t.Run(name, func(t *testing.T) { stdReproNamed(t, name) })
	}
}
