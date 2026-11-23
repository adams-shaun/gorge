package oracle

import "testing"

// The dr-mbc-aaab evidence sweep's pair 94 (seed 222108), replayed on a table
// built from its own decks (pool "custom"): at p0's priority, gorge's autopay
// vocabulary carries only the casts its planner pays (Vanish into Eternity's
// own RaiseCost ValidTarget$ Creature declines the planner's shape, and its
// potential section is dropped) and the kernel must too. The kernel once
// offered the plain cast (gorge 6 casts, kernel 7).
//
// vanish_into_eternity_raised_cost_offer (pair 181, seed 310019) is the
// converse: p1's only sources are Desert of the Fervent, a Forest and a Lotus
// Petal, which is also the only nonland permanent, so the cast is payable only
// by sacrificing its own target and gorge's planner prices it (targetPaid, the
// cheapest target set's plan). The kernel's target-paid witness planned for no
// spell, so pair 94's Powerstone {C} (a non-spell restricted source) joined
// its plan, floated restricted, and the witness read "the pool does not pay" in
// the one case and the blanket refusal hid the cast in the other.
func TestMBCVanishIntoEternityReprosLockstep(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"tgt_raise_cast_offer", "vanish_into_eternity_raised_cost_offer"} {
		t.Run(name, func(t *testing.T) { stdReproNamed(t, name) })
	}
}
