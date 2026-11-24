package oracle

import "testing"

// CR 605.3a / CR 602.2b: Radiant Solar's "{W}, Discard this card" is payable
// from a Henge of Ramos ({2},{T}: one mana of any colour) funded by two
// Mountains, while Mercadian Bazaar sits untapped beside them. Both engines
// must agree on whether the hand activation is offered (decision 179 of the
// tail-26-c001 autopay game, pair 45).
func TestMBCRadiantSolarReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "radiant_solar_henge_fee_offer")
}
