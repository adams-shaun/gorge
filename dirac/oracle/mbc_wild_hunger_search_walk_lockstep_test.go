package oracle

import "testing"

// Wild Hunger's flashback {3}{R} beside a granted "{T}: Add one mana of any
// color" (Abundant Growth) on a board whose tap choices sit near gorge's
// prefix-search node budget: gorge's search itself decides the offer, so
// dirac must run the same walk and not a count of tap choices (tail-15,
// pair 44, decision 648: gorge's search finishes and offers).
func TestMBCWildHungerSearchWalkOffersReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "wild_hunger_flashback_search_walk_offers")
}
