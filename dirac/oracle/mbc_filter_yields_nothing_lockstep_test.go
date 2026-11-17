package oracle

import "testing"

// A filter whose dynamic amount resolves to zero makes nothing (Baldur's
// Gate's "{2}, {T}: Add X mana of any one color, X is the number of other
// Gates you control" beside no other Gate), so gorge's census lists no
// ability of it (manaAbilityYieldsNothing) and the card is only its plain
// "{T}: Add {C}" source. The kernel classified the zero-amount ability as a
// filter (gfFilt), so its prefix search funded the zero filter's fee for no
// mana -- the fee funding is one covered activation per fee-bearing ability
// -- and the search ran out of budget before it paid the hand cast, which
// gorge offered through its planner (decision 1142 of tail-51 pair 4).
func TestMBCFilterYieldsNothingReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_yields_nothing")
}
