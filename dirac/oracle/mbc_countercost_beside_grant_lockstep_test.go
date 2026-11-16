package oracle

import "testing"

// CR 601.2g-h with CR 605.3b: gorge's census lists neither a granted mana
// ability (Jaheira's "{T}: Add {G}" on a Squirrel token) nor a counter-cost
// one (Wall of Roots' "Put a -0/-1 counter on Wall of Roots: Add {G}"), so a
// play the planner cannot price is one prefix search over every gap in
// battlefield order: the token's {G} before the wall's. The kernel's grant
// search listed no counter-cost candidate and applyPriority's grant-first arm
// ignored the gap, so the counter-cost script ran ahead of the granted
// source and spent the wall's counter where gorge tapped the token.
func TestMBCCounterCostBesideGrantReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "countercost_gap_beside_grant_one_search")
}

// The same one search prices the offer: a Colossal Dreadmaw only the four
// tokens and both walls pay together is on gorge's list. scriptOffers' tap-
// another branch (which counts the counter-cost gaps) searched without the
// grants and dropped the play unless a tap-another source was live beside
// them.
func TestMBCCounterCostBesideGrantOffersLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "countercost_gap_beside_grant_offers_cast")
}

// A counter-cost source with a plain {T} first ability (Vivid Grove) poses
// that ability's answer before the counter one in gorge's activation ask, so
// the search's step for it taps the printed {G} first and removes no charge
// counter unless the later answers need it.
func TestMBCCounterCostSecondAbilityBesideGrantReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "countercost_second_ability_beside_grant_printed_first")
}
