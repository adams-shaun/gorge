package oracle

import "testing"

// TestReproKCIDepletionLandLockstep replays the recorded Krark-Clan Ironworks
// games: a no-{T} sacrifice-an-artifact mana source beside a depletion land
// (the MBC pool's SubCounter riders, Peat Bog and Sandstone Needle among them)
// offers and pays the same on both sides — the tap-another search branches the
// deferred gap like gorge's one search. The
// recordings were the MBC run's divergent pairs 9 and 31 (gorge offers the cast
// the kernel did not) and 41, 46 and 48 (the two searches paid with different
// sources, the depletion land sacrificed in gorge, Krark-Clan Ironworks in its
// graveyard); each replays clean now. They play the "custom" table their own
// decks need: the MBC pool splits by hash, so no one split holds every card of
// a game, and a recording must not name a split.
func TestReproKCIDepletionLandLockstep(t *testing.T) {
	for _, name := range []string{
		"sacrifice_mana_pays_beside_depletion_land",
		"sacrifice_mana_pays_beside_depletion_land_second",
		"sacrifice_mana_and_depletion_land_pay_one_search_projection",
		"sacrifice_mana_and_depletion_land_pay_one_search_projection_second",
		"sacrifice_mana_and_depletion_land_pay_one_search_projection_third",
		// dr-mbc-bbbb pair 65: an untapped Treasure opens gorge's potential
		// bound, so its walk lists the second Ironworks and the search pays it
		// by sacrificing the Treasure, then the Ironworks itself
		"sacrifice_mana_sacrifices_treasure_then_itself",
		// dr-mbc-bbbb pair 58: the sacrifice-another source and the granted
		// mana ability pay one cast together (Abundant Growth on an untapped
		// Forest beside two untapped Ironworks, a Brood Monitor in hand); the
		// grant search lists the sacrifice gap as a candidate (mana_grantsacx.go)
		"sacrifice_mana_and_granted_mana_pay_a_cast",
		// dr-mbc-bbbb pair 23: a {4} cast beside the untapped Ironworks and an
		// untapped Fellwar Stone, the potential bound's B1 R1 + the Ironworks'
		// {C}{C} listing it and the search paying it with two activations (The
		// Rack, then the Fellwar Stone); the potential bound admits the
		// sacrifice-another gap (legal.go potentialGapCost) and the candidates
		// are the bound's plays (script_gen.go potentialBoundLists)
		"sacrifice_mana_gap_joins_the_potential_bound",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			stdReproNamed(t, name)
		})
	}
}
