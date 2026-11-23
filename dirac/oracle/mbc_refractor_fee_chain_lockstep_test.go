package oracle

import "testing"

// CR 605.3a: Energy Refractor's "{2}: Add one mana of any color" is a mana
// ability, so two Mountains pay its fee and its black funds the second {B} of
// Shatter Assumptions ({1}{B}{B}) beside a Swamp and a Mystic Gate's {C}. gorge
// offered the cast; the kernel's offer lacked it (tail-02 pair 8, decision 185).
func TestMBCShatterAssumptionsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "refractor_fee_chain_funds_black_pips")
}
