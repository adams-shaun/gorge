package oracle

import (
	"path/filepath"
	"testing"
)

const sacAnyScriptRepro = "sac_any_script_beside_lethal_deferred_gap_overoffers"

// Ashnod's Altar's "Sacrifice a creature: Add {C}{C}" beside a Crumbling
// Necropolis deferred by p1's Burning Earth (pool tail-71, surface full, the
// recorded shrunk game; gorge ids 3/1): gorge's one prefix search over the two
// census gaps walks the deferred land first, its tap's lethal trigger waits on
// the stack while the repeatable sac-any asks branch per creature, and every
// priority node of that subtree has a pass child whose resolving trigger kills
// p0 (life 1). gorge counts that dead child as a node (scriptSearch.visit counts
// before it sees the game over), so a small node budget dies before the
// Altar-only line is reached and it offers no Spiked Ripsaw equip at decision
// 614 (CR 603.2, 104.3a, 605.3b). The kernel's tap-another search did not tick
// for the dead child and priced the equip at its 136th node.
//
// At the shared potentialScriptNodes budget gorge reaches Drana, Kalastria
// Bloodchief's cast too (two Altar sacrifices and the land, 190 nodes). The
// kernel's any-mana candidate walk withheld that cast at 1 life under Burning
// Earth (plainCastPainOK), where the gaps walk lists it, so it never searched
// it; on a closed sacrifice-any bound the candidates are both walks' plays
// (unionGapsWalk).
func TestMBCSacAnyScriptBesideLethalDeferredGapLockstep(t *testing.T) {
	r, err := ReadRepro(filepath.Join("testdata", "repro", sacAnyScriptRepro+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	res, _, err := replayOnOwnCards(t, r)
	if err != nil {
		t.Fatal(err)
	}
	if res.Div != nil || res.StallUnmodelled {
		t.Fatalf("the recorded game diverges at decision 614: %+v (unmodelled %v)", res.Div, res.StallUnmodelled)
	}
	if res.Decisions < len(r.Actions) {
		t.Fatalf("the replay ended at decision %d of %d recorded: the equip decision was never reached", res.Decisions, len(r.Actions))
	}
}
