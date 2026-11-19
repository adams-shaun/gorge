package oracle

import "testing"

// Bash to Bits' flashback ({4}{R}{R}) in a priority decision with Energy
// Refractor's repeatable fee-only "{2}: Add one mana of any color" beside a
// full battlefield (decision 393 of 393, tail-02-c000 pair 55, seed 648290):
// gorge and the kernel must agree on whether the cast is offered. Both
// prefix searches are node-budgeted, and the fee-only gap's funding spends
// that budget on the filter's colour branches; gorge's census skipped that
// funding when Mystic Gate's unrelaxable gap came first, so it priced the
// cast where the kernel did not. paymentPlanCensusOf now funds the later
// repeatable fee-only gap too (CR 605.3a), and the two searches walk the
// same space. Reverting that hunk fails this test.
func TestMBCBashToBitsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "flashback_cast_priced_over_ability_cost")
}
