package oracle

import "testing"

// CR 118.3: Darba's upkeep "sacrifice Darba unless you pay {G}{G}" must offer
// the pay arm when a paid-life painland can make the second {G}. The recording
// is decision 372 of tail-16 pair 0, where Hashep Oasis's "{T}, Pay 1 life: Add
// {G}" funds the cost and gorge offered pay beside the decline while the
// kernel offered the decline alone.
func TestMBCUnlessPayPainlandOffersPayReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_pay_painland_offers_pay")
}
