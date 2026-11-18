package oracle

import "testing"

// Treasonous Ogre's only mana ability is "Pay 3 life: Add {R}" — no {T}. The
// gorge planner prices it as the source's last-resort unit with
// Consequence{Life: 3} (pay.PaymentPlanShapeTierOf's cost.Life arm asks no
// {T}), so the {4}{R} cast is affordable and the priority options carry it.
// The kernel priced the same ability as a census gap (kop.GapLifeManaAb), a
// unit the autopay surface drops with every plain cast it funds, so its
// options held only pass (decision 498; dr-mbc-bbbb pair 69, seed 550498).
// The kernel now joins a SOLE life-only ability to the pain census
// (dirac mana_pain.go prepSoleLifeMana) the same way a printed
// sacrifice-only ability joins it (mana_sacsource.go).
func TestMBCTreasonousOgreReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "life_only_mana_ability_prices_the_cast")
}
