package oracle

import (
	"testing"
)

// CR 601.2g-h with CR 605.3a: while paying a spell, a mana ability's own
// activation fee may be paid from mana already in the pool. p1's only fifth
// mana at the recorded decision is Shadowblood Ridge's "{1}, {T}: Add {B}{R}"
// (kop.FilterManaAb's Skycloud Expanse shape): a once-only paid mana ability
// whose fee the census sources fund while it taps. Ichorid {3}{B} and
// Oathsworn Knight {1}{B}{B} are offered by both engines; the fifth mana is
// exactly the Ridge's.
//
// The kernel priced the Ridge and its filter route did offer the cast on a
// filter-only board, but the game's board also holds Untaidake, the Cloud
// Keeper ({T}, Pay 2 life: Add {C}{C}, spend only on legendary spells) — a
// restricted planner unit (mana_restrictedcost.go), whose census prices it
// for exactly the payments its restriction admits. Beside it the filter
// route was gated off (mana_filter.go gapOthers kept the restricted unit a
// census gap, so filtOnly was false; mixBoard counted no second gap kind),
// so no kernel route searched the Ridge and the gaps walk priced the cast
// without its units, while gorge's one prefix search (potentialPrefixScript)
// activated the Ridge, funded its {1} from a floated Forest, and offered.
// The fix (censusGapMana leaves restricted planner units to the census, so a
// filter beside one is filter-only again) makes this recording replay clean.
func TestMBCSkyshroudVampireReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "skyshroud_vampire_priority_cast_offer")
}
