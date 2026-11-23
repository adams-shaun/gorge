package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const rayOfRevelationRepro = "grove_gap_flashback_script_budget"

// CR 605.3b, 601.2g: Ray of Revelation's flashback {G} is payable by Grove of the
// Burnwillows' "{T}: Add {R} or {G}" (a census gap through its opponent-gains-1-life
// rider) with Racecourse Fury a legal target. Darkwater Catacombs' {1},{T} filter
// and Gemstone Caverns' counter-conditioned mana are two more gaps, so gorge's
// prefix search walks their colour and funding branches, exhausts its 160-node
// budget (164 nodes, limited, no script) and prices the cast census_incomplete: it
// offers pass and two activations at decision 1854, while the kernel also offers
// the cast (tail-04-c000 pair 3, seed 755213). Matching gorge's exhaustion node for
// node is beyond bounded dirac code, so the card is refused by name and its
// recorded game still diverges with it implicated.
func TestMBCRayOfRevelationReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range entries {
		e := &entries[i]
		if e.Card != "Ray of Revelation" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != rayOfRevelationRepro {
			t.Fatalf("Ray of Revelation's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, rayOfRevelationRepro)
		}
		checkRefusedRepro(t, e)
		found = true
	}
	if !found {
		t.Fatal("Ray of Revelation has no dirac/support/refused entry: its flashback cast is priced by a prefix search that gorge's three census gaps exhaust")
	}
}
