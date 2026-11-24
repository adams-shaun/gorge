package oracle

import "testing"

const bestialMenaceRepro = "prefix_search_budget_sac_grants_menace"

// Rain of Filth gives the tapped lands "Sacrifice this land: Add {B}" and The
// World Tree (six or more lands) gives the untapped lands "{T}: Add one mana of
// any color". Bestial Menace costs {3}{G}{G}: tap The World Tree and the untapped
// Swamp for {G}{G} and sacrifice three tapped lands for {B} would pay it (CR
// 605.3a, 106.1b). gorge's prefix search finds no script over those grants
// within its 160-node budget (potentialScriptNodes), so the cast is
// census_incomplete and never offered (tail-08-c000 pair 94, decision 623). The
// kernel's grant search used to keep the grant lists it read at the root through
// a sacrifice step and priced the cast below the sacrificed World Tree; it now
// re-reads each source's granted abilities at the sacrifice steps
// (mana_grantcost.go sacBranch and tapSacBranch), so it withholds the cast like
// gorge. The recorded game replays clean and the card is unrefused.
func TestMBCBestialMenaceReprosLockstep(t *testing.T) {
	t.Parallel()
	oneOffReproClean(t, bestialMenaceRepro)
}
