package oracle

import "testing"

// CR 602.2 with CR 118.12: Rhystic Study's "may pay {1}" is offered to the
// caster only when the {1} can be paid. Its only untapped source is Shadowblood
// Ridge ({1}, {T}: Add {B}{R}), whose own {1} fee nothing else can fund, so
// gorge offers "don't pay" alone; dirac counted the Ridge's tap as a unit and
// offered "pay" too (decision 541, tail-73 pair 18, Bedlam Reveler).
func TestMBCBedlamRevelerUnlessFeeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_cost_counts_mana_fee_source")
}
