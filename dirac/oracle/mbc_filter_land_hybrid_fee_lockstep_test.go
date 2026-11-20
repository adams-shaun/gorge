package oracle

import "testing"

// CR 601.2f, 605.3a: Stolen by the Fae ({X}{U}{U}) with X=1 against Foundry
// Street Denizen, paid from Island (the filter land's {G/U} fee), Flooded
// Grove ({U}{U}) and a Plains ({1}). gorge offers the cast; dirac did not.
func TestMBCStolenByTheFaeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_land_hybrid_fee_x_spell")
}
