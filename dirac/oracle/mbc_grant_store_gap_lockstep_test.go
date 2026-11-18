package oracle

import "testing"

// CR 601.2g-h, 605.1a: The World Tree ({W}{W}{U}{U}{B}{B}{R}{R}{G}{G}, {T},
// sacrifice) beside twelve lands it grants "{T}: Add one mana of any color"
// and a Dwarven Hold holding storage counters. gorge's one prefix search
// activates every gap, the grants and the battery together, and spends its
// 160 nodes on the ten pips (census_incomplete), so the ability is not
// offered. The kernel's grant search spends the same budget and fails, but
// then left the play to the counter-gap search because a counter-cost gap
// was live, and that search cannot price the grants: it kept the play on
// the offer unfunded (decision 648 of 648, tail-08 pair 10, autopay).
func TestMBCTheWorldTreeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "grant_search_spent_beside_storage_gap")
}
