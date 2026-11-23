package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const fireDragonRepro = "chromatic_star_gap_script_budget_exhausted"

// CR 605.3b, 601.2g: Fire Dragon ({6}{R}{R}{R}) is payable by nine mana:
// four Forests, three Mountains and a Forest wearing A-Find the Path ("{T}: Add
// two mana of any one color"). Gorge's planner prices no granted ability, so the
// cast is found by its prefix search, and Chromatic Star's {1},{T},sacrifice
// filter (funding one source per activation) is a census gap beside the grant:
// the search walks the Star's colour and funding branches, exhausts its
// 160-node budget (plain and phased) with no script, and offers pass and Enormous
// Baloth alone at decision 450 (tail-62 pair 22, seed 489595). Without the Star
// gap the same search finds the three-step script; the kernel offers the cast.
// Matching gorge's exhaustion node for node is far past bounded dirac code, so
// Fire Dragon is refused by name and its recorded game still diverges with it
// implicated.
func TestMBCFireDragonReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range entries {
		e := &entries[i]
		if e.Card != "Fire Dragon" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != fireDragonRepro {
			t.Fatalf("Fire Dragon's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, fireDragonRepro)
		}
		checkRefusedRepro(t, e)
		found = true
	}
	if !found {
		t.Fatal("Fire Dragon has no dirac/support/refused entry: its cast is priced by a prefix search that gorge's Chromatic Star gap exhausts")
	}
}
