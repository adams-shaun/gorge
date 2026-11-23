package oracle

import "testing"

// CR 605.3a with CR 107.4b and CR 118.12: Minion of Tevesh Szat's "unless you
// pay {B}{B}" upkeep tax is payable by tapping Fetid Heath, whose "{W/B}, {T}:
// Add {B}{B}" activation is funded by any white or black mana (here a Plains).
// The gorge offer gate priced no hybrid-fee mana activation and posed only the
// decline option where dirac offered pay as well (decision 444 of 444, tail-01
// pair 20, mode(-,-,1) only one engine offers).
func TestMBCRathiTrapperReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_pay_hybrid_fee_filter_land")
}
