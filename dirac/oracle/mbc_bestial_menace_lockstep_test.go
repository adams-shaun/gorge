package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const bestialMenaceRepro = "prefix_search_budget_sac_grants_menace"

// Rain of Filth gives the tapped lands "Sacrifice this land: Add {B}" and The
// World Tree (six or more lands) gives the untapped lands "{T}: Add one mana of
// any color". Bestial Menace costs {3}{G}{G}: tap The World Tree and the untapped
// Swamp for {G}{G} and sacrifice three tapped lands for {B} pays it (CR 605.3a,
// 106.1b). gorge's prefix search finds no script over those grants within its
// 160-node budget (potentialScriptNodes), so the cast is census_incomplete and
// never offered; the kernel prices it from the grants and offers it
// (tail-08-c000 pair 94, decision 623). Matching the search's node accounting
// over a dynamic grant list is far more than the kernel's script mirror holds,
// so the card is refused by name as Iron Fist, Hero for Hire is, and the
// recorded game still shows the divergence with it implicated.
func TestMBCBestialMenaceReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Bestial Menace" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != bestialMenaceRepro {
			t.Fatalf("Bestial Menace's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, bestialMenaceRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Bestial Menace has no dirac/support/refused entry: gorge's prefix search finds no script over the granted sacrifices and the granted any-colour taps and the kernel offers the cast")
}
