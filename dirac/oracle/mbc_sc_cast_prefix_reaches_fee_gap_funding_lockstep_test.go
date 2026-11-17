package oracle

import "testing"

// CR 601.2b: a mana ability whose activation itself costs mana, such as
// Crossroads Candleguide's "{2}: Add one mana of any color", is activated
// with the mana floating at the moment it resolves, so a cast funded through
// it is legal: the funding taps float first, each {2} drains the floats, the
// ability's mana floats on. Spontaneous Combustion ({1}{B}{R}, sacrifice a
// creature -- Candleguide itself, after its ability floated) needed exactly
// that line on the recorded board, and gorge's plain prefix search spent its
// node budget on the fee gap's dead branches before it, so gorge withheld
// the cast the kernel offered.
// The recording is on a custom pool, the table its own decks need: the tail
// buckets a synthetic pool deals re-cut whenever the lowered card set moves.
func TestMBCSpontaneousCombustionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sc_cast_prefix_reaches_fee_gap_funding")
}
