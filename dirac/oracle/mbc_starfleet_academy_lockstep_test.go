package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const starfleetAcademyRepro = "paid_dynamic_mana_source_cast_offer"

// CR 601.2g, 605.3b: five Forests pay Subcommander T'Pol's {2}{U} through
// Starfleet Academy's "{5},{T}: Add X mana of one chosen colour" (X the
// creature types among non-Borg creatures you control). Gorge's prefix search
// exhausts its 160-node budget on the plain and the phased runs (the paid
// dynamic gap's fee branches over the Forests and an Orion land) and withholds
// the cast (decision 305 of std-trk pair 41); the kernel's open any-colour
// production offers it. Mirroring the budgeted node walk is past a bounded
// change, so the card is refused by name and the recorded game still shows the
// divergence with it implicated.
func TestMBCStarfleetAcademyReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Starfleet Academy" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != starfleetAcademyRepro {
			t.Fatalf("Starfleet Academy's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, starfleetAcademyRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Starfleet Academy has no dirac/support/refused entry: gorge withholds the cast its dynamic gap funds, the kernel offers it")
}
