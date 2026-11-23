package oracle

import "testing"

// CR 601.2g with CR 605.3c: a repeatable mana ability whose activation cost
// taps another permanent (Urza, Lord High Artificer's "Tap an untapped artifact
// you control: Add {U}") is one legal activation per untapped candidate, so a
// {2} activation beside two untapped artifacts (Jeweled Lotus's commander-only
// tap still taps, beside Urza's own Construct token) is payable and must be
// offered. The payment-plan census priced the ability as one unit -- one {U} --
// so the Genju of the Spires {2} activation was offered by the kernel and
// withheld by gorge (decision 642 of the tail-64-c001 pair 9 game the evidence
// loop parted on).
func TestMBCGenjuOfTheSpiresReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "tapanother_census_one_unit")
}
