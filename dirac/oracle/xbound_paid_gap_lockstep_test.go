package oracle

import "testing"

// CR 601.2g-h with CR 605.3b: Golgari Signet's "{1},{T}: Add {B}{G}" spends a
// mana to make two, and its own output cannot pay that fee. Beside one
// untapped Island, Repeal ({X}{U}) aimed at the only mana value 1 permanent
// costs {1}{U}: the Island pays the {U} or the Signet's {1}, never both. The
// relaxed payment proof once priced the Signet's {B}{G} as free units beside
// the Island and charged the fee on top, so gorge offered that cast and the
// kernel withheld it.
func TestXBoundPaidGapSelfFundedFeeLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "xbound_paid_gap_self_funded_fee")
}
