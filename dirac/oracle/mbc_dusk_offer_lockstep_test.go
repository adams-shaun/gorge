package oracle

import "testing"

// CR 605.1a: Palladium Myr under Cryptolith Rite taps once, for {C}{C} or for one mana of any
// colour, so two Forests and the Myr hold one white source and Dusk's {2}{W}{W} is not
// payable. The kernel's census read the Myr as two mana of any colour and offered the cast
// gorge does not (decision 323, tail-15-c000 pair 4).
func TestMBCDuskReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "dusk_offer_split_card")
}
