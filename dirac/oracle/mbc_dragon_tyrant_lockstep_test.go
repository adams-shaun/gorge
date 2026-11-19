package oracle

import "testing"

// Dragon Tyrant's upkeep trigger sacrifices it unless its controller pays
// {R}{R}{R}{R} (tail-16-c000 pair 70). Gorge offers pay and decline, its
// census pricing Grand Coliseum's any colour; dirac offered decline only
// (decision 826). Paying, gorge's window answers Graven Cairns' hybrid filter
// for {B}{B} and Grand Coliseum's {C}, comes up short and sacrifices it;
// dirac tapped Cairns for {C} and Coliseum for {R} with pain, and paid
// (decision 827).
func TestMBCDragonTyrantReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_mana_cost_payable_from_pain_and_filter_lands")
}
