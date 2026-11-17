package oracle

import (
	"testing"
)

// CR 605.3b with the payment planner's lethal guard (the cast-payment-plans
// design §5: a plan whose summed life and damage would reduce the caster to 0
// or less is never offered): Kytheon's Irregulars' {W}{W}: Tap target creature.
// is unpayable
// when its controller sits at 1 life and the only untapped white sources
// beside one Plains is a City of Brass, whose Taps trigger would deal the
// killing point. gorge's planner proves the cost unpayable and withholds the
// ability; the kernel's activation offer read its availDynSac census mode
// (bit 64) as the manual cast window's availWinCol, so availFor priced the
// City of Brass with no lethal guard and offered the activation
// (tail-57-c005 pair 116, decision 618).
func TestMBCKytheonSIrregularsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "kytheons_irregulars_lethal_pain_offer")
}
