package oracle

import "testing"

const worldTreePrefixBudgetRepro = "world_tree_any_colour_prefix_budget"

// CR 106.1, 605.1a, 601.2g: The World Tree's "lands you control have '{T}: Add
// one mana of any color'" (six or more lands) is the only blue p0 can reach
// once a Rain of Filth turn has made every other land sacrifice for {B}, so
// O'aka, Traveling Merchant ({1}{U}) and Wormhole Serpent ({4}{U}) would be
// payable by tapping Swamp or Dwarven Hold for {U} and sacrificing lands for
// the rest. Gorge's potentialPrefixScript walks the sacrifice answers first and
// spends its 160-clone budget before it reaches the granted any-colour answer,
// so both casts are priced census_incomplete and not offered (tail-08 pair 8,
// decision 473). The kernel's grant search used to keep the grant lists it read
// at the root through a sacrifice step and offered them; it now re-reads each
// source's granted abilities at the sacrifice steps (mana_grantcost.go
// sacBranch and tapSacBranch), so it withholds them like gorge. The recorded
// game replays clean and both cards are unrefused.
func TestMBCOAkaTravelingMerchantReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, worldTreePrefixBudgetRepro)
}
