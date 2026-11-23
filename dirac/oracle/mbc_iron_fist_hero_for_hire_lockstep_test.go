package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const ironFistHeroForHireRepro = "prefix_search_budget_granted_any_with_sac_grants"

// Rain of Filth gives the eight tapped lands "Sacrifice this land: Add {B}", and
// The World Tree (nine lands, six or more) gives the one untapped Swamp "{T}:
// Add one mana of any color". Iron Fist, Hero for Hire costs {1}{R}: tap the
// Swamp for {R} and sacrifice any land for {B} pays it (CR 605.3a, 106.1b).
// gorge's prefix search (potentialPrefixScript) finds no script over those
// grants within its 160-node budget (potentialScriptNodes), so the cast is
// census_incomplete and never offered, while
// Dread Specter and Heirloom Blade, payable by sacrifices alone, are. The kernel
// prices {R} from the granted tap and offers it (tail-08-c000 pair 43, decision
// 637). Matching the search's node accounting over a dynamic grant list is far
// more than the kernel's script mirror holds, so the card is refused by name as
// Captain America, Living Legend is, and the recorded game still shows the
// divergence with it implicated.
func TestMBCIronFistHeroForHireReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Iron Fist, Hero for Hire" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != ironFistHeroForHireRepro {
			t.Fatalf("Iron Fist, Hero for Hire's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, ironFistHeroForHireRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Iron Fist, Hero for Hire has no dirac/support/refused entry: gorge's prefix search finds no script over the granted sacrifices and the granted any-colour tap and the kernel offers the cast")
}
