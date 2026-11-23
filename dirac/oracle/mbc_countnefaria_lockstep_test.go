package oracle

import "testing"

// CR 106.6, 601.2f-g: Count Nefaria costs {3} less once its controller has
// sacrificed a permanent this turn, but Master of Dark Rites' {B}{B}{B} (paid
// by sacrificing another creature) is spendable only on Vampire, Cleric and
// Demon spells, so it can pay nothing of a Villain's cost. Gorge's prefix
// search never activates a gap the searched cast cannot spend
// (scriptSpendableGaps), so with six lands against the full {6}{B} the cast
// is not offered; the kernel's tap-another search activated Master anyway,
// let the sacrifice discount the cost and offered it (tail-62-c000 pair 25,
// decision 527).
func TestMBCCountNefariaReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "restricted_sacx_gap_unspendable_for_discounted_cast")
}
