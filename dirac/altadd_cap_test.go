package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestGrantedManaScriptCapCountsAnEitherOrAdditionalCostCast: gorge's prefix search for a
// granted mana ability is spent on an either-or additional cost's cast like any other
// play its printed sources cannot pay, so with one Deadly Precision more than
// potentialScriptPlays in hand, each payable only by Great Divide Guide's granted {B}, the
// first potentialScriptPlays are offered and the last is not. A hand this large is not a
// legal deck: the scenario only needs the walk's length.
func TestGrantedManaScriptCapCountsAnEitherOrAdditionalCostCast(t *testing.T) {
	deck := []cardset.Entry{{Name: "Deadly Precision", Count: potentialScriptPlays + 1}, {Name: "Great Divide Guide", Count: 1}, {Name: "Forest", Count: 4}}
	set := cardsettest.Set(t, "grantsac")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the grantsac table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	{
		// the autopay surface drops a plain cast a prefix pays, so only the full surface lists them
		sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
		for i := 0; i < potentialScriptPlays+1; i++ {
			sc.hand(id["Deadly Precision"], 0)
		}
		sc.perm(id["Forest"], 0)
		sc.perm(id["Great Divide Guide"], 0)
		sc.perm(id["Great Divide Guide"], 1)
		d := sc.s.Step()
		n := 0
		for _, a := range d.Options[:d.N] {
			if a.Kind() == AkCast {
				n++
			}
		}
		if n != potentialScriptPlays {
			t.Errorf("%d Deadly Precision casts offered, want %d (%v)", n, potentialScriptPlays, d.Options[:d.N])
		}
	}
}
