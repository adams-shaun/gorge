package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const bogardanPhoenixRepro = "bogardan_phoenix_granted_tap_gold_budget"

// CR 602.2b, 605.3a: Bogardan Phoenix ({2}{R}{R}{R}) is payable from a
// Mountain, two Forests, Painted Bluffs ({T}: {C}; {1}, {T}: one mana of any
// color) and a Gold token (sacrifice: one mana of any color) once Jaheira,
// Friend of the Forest gives the token "{T}: Add {G}". That granted tap makes
// the Gold token a census gap beside the Bluffs, and gorge's plain prefix
// search needs 372 nodes against its 160-node budget, so the play is left
// "unsupported census_incomplete" and the full surface withholds it while the
// kernel prices and offers it (tail-76 pair 183, decision 291, "options
// options"). The search is not modelled in the kernel, so the card is refused
// by name and the recorded game still shows the divergence with it implicated.
func TestMBCBogardanPhoenixReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Bogardan Phoenix" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != bogardanPhoenixRepro {
			t.Fatalf("Bogardan Phoenix's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, bogardanPhoenixRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Bogardan Phoenix has no dirac/support/refused entry: gorge's prefix search gives up on a granted-tap Gold token beside Painted Bluffs")
}
