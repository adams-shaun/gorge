package oracle

import "testing"

const namorRepro = "fee_any_mana_ability_funds_a_cast_the_kernel_census_misses"

// CR 601.2g, 605.3a-b: Namor the Sub-Mariner's {1}{U}{U} is payable when Baxter
// Building's "{4},{T}: Add four mana in any combination of colors" funds the
// second {U} as a fee in the cast window. gorge prices that fee and offers the
// cast; the kernel's census once priced Baxter at its cheapest "{T}: Add {C}" and
// offered none. The kernel's filter prefix search now takes the fee-order
// fallback (617e139cf3, dirac/mana_filter.go, dirac/mana_filterfeeorder.go) and
// funds the fee, so on the pinned game both engines offer the cast and the
// recorded game replays clean. The card is therefore no longer refused
// (dirac/support/unrefused.tsv); this test pins the unrefusal's proof.
func TestNamorFeeFundedCastReproReplaysClean(t *testing.T) {
	res := replayRawRepro(t, namorRepro)
	if res.Div != nil {
		t.Fatalf("%s's recorded game diverges at %d: %s", namorRepro, res.Div.At, res.Div)
	}
	if res.StallUnmodelled {
		t.Fatalf("%s's recorded game ends at an unmodelled stall: %q", namorRepro, res.Stall)
	}
}
