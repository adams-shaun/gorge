package oracle

import "testing"

// Dominate ({X}{1}{U}{U}, "target creature with mana value X or less") with
// an untapped board that pays some X leaving a creature legal (CR 601.2b,
// 601.2c): gorge offers the cast at priority, dirac didn't (decision 445 of
// 445, tail-66 pair 2 of seed 791080).
func TestMBCDominateXBoundOfferReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "x_bound_spell_offer_dominate")
}
