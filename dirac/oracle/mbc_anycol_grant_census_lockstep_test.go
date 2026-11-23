package oracle

import "testing"

// Aberrant Return, exiled off the top by Chase Stein, Runaway ("until the end of your next turn,
// you may play that card"), is offered as a cast by dirac at decision 522 of tail-08-c000 pair 44
// while gorge offered only pass: dirac's plain payment census credited The World Tree's static
// grant ("lands you control have '{T}: Add one mana of any color'", live at seven lands) as a
// spendable unit on every land, where gorge's payment planner prices no granted mana ability
// (pay.PaymentAbility holds grants out of the census's alternatives).
func TestMBCAberrantReturnReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "anycol_grant_census")
}
