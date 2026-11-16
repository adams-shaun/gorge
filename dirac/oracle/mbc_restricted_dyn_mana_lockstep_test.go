package oracle

import "testing"

// CR 106.6, 605.1a: Vhal, Candlekeep Researcher's restricted {T} ability adds
// {C} equal to its toughness. The planner counted that evaluated amount as
// that many mana of one colour of the payer's choosing (Helga's shape), so a
// second {U} activation of Watercourser looked payable with every land tapped
// and the kernel offered it while gorge offered only pass (decision 409,
// tail-44 pair 110).
func TestMBCWatercourserReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "restricted_fixed_colour_dyn_mana")
}
