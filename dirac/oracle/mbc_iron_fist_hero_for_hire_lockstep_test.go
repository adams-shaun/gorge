package oracle

import "testing"

const ironFistHeroForHireRepro = "prefix_search_budget_granted_any_with_sac_grants"

// Rain of Filth gives the eight tapped lands "Sacrifice this land: Add {B}", and
// The World Tree (nine lands, six or more) gives the one untapped Swamp "{T}:
// Add one mana of any color". Iron Fist, Hero for Hire costs {1}{R}: tap the
// Swamp for {R} and sacrifice any land for {B} would pay it (CR 605.3a, 106.1b).
// gorge's prefix search (potentialPrefixScript) finds no script over those
// grants within its 160-node budget (potentialScriptNodes), so the cast is
// census_incomplete and never offered, while Dread Specter and Heirloom Blade,
// payable by sacrifices alone, are (tail-08-c000 pair 43, decision 637). The
// kernel's grant search used to keep the grant lists it read at the root through
// a sacrifice step and priced {R} from the granted tap below the sacrificed
// World Tree; it now re-reads each source's granted abilities at the sacrifice
// steps (mana_grantcost.go sacBranch and tapSacBranch), so it withholds the cast
// like gorge. The recorded game replays clean and the card is unrefused.
func TestMBCIronFistHeroForHireReprosLockstep(t *testing.T) {
	t.Parallel()
	oneOffReproClean(t, ironFistHeroForHireRepro)
}
