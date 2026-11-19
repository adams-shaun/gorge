package oracle

import "testing"

// Darba's "sacrifice it unless you pay {G}{G}" is offered a pay answer by
// gorge when the payer's only way to the second {G} is Grand Coliseum's
// painful any-colour ability beside a Forest (CR 605.3a, 118.3: the 1 damage is
// no cost, so the unless cost is payable). Recorded as tail-16-c000 pair 75
// (autopay, seed 309962).
func TestMBCAProsperousThiefReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_pay_pain_any_color_land")
}
