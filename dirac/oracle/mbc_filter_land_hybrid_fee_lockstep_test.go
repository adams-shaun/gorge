package oracle

import "testing"

// CR 601.2f, 605.3a: Stolen by the Fae ({X}{U}{U}) with X=1 against Foundry
// Street Denizen, paid from Island (the filter land's {G/U} fee), Flooded
// Grove ({U}{U}) and a Plains ({1}). gorge offers the cast; dirac did not.
//
// CR 107.3b, 601.2c: the second recording is the same card at X=0 (dr tail-15-c000
// pair 31, decision 267): the cast is offered at priority in one engine only.
func TestMBCStolenByTheFaeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_land_hybrid_fee_x_spell")
	stdReproNamed(t, "stolen_by_the_fae_x_zero_cast_offer")
}
