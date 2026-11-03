package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestGrantedManaScriptCapCountsCastsUnderAReader: a mana-spent reader on the
// other player's battlefield makes gorge's cast planner decline every cast
// ("shape:mana_spent_reader"), but the potential planner prices the cast from
// its composed cost all the same, so each such cast the printed sources cannot
// pay takes one of the prefix searches. With the reader out, one Deadly Embrace
// more than potentialScriptPlays, payable only by the granted any-colour mana, is
// offered up to the cap, as without it; before, the kernel counted none of them.
// A hand this large is not a legal deck: the scenario only needs the walk's length.
func TestGrantedManaScriptCapCountsCastsUnderAReader(t *testing.T) {
	deck := []cardset.Entry{{Name: "Deadly Embrace", Count: potentialScriptPlays + 1}, {Name: "A Realm Reborn", Count: 1}, {Name: "Tellah, Great Sage", Count: 1},
		{Name: "Namazu Trader", Count: 5}, {Name: "Swamp", Count: 4}}
	set := cardsettest.Synth(t, "std-fin")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the std-fin table", e.Name)
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
	for _, tc := range []struct {
		reader  bool
		offered int
	}{{false, potentialScriptPlays}, {true, potentialScriptPlays}} {
		sc := newScenTableLib(t, tbl, id["Swamp"], Config{NoElide: true})
		for i := 0; i < potentialScriptPlays+1; i++ {
			sc.hand(id["Deadly Embrace"], 0)
		}
		sc.perm(id["Swamp"], 0)
		sc.perm(id["Swamp"], 0)
		sc.perm(id["A Realm Reborn"], 0)
		for i := 0; i < 3; i++ {
			sc.perm(id["Namazu Trader"], 0)
		}
		sc.perm(id["Namazu Trader"], 1)
		if tc.reader {
			sc.perm(id["Tellah, Great Sage"], 1)
		}
		d := sc.s.Step()
		n := 0
		for _, a := range d.Options[:d.N] {
			if a.Kind() == AkCast {
				n++
			}
		}
		if n != tc.offered {
			t.Errorf("reader out %v: %d Deadly Embrace casts offered, want %d (%v)", tc.reader, n, tc.offered, d.Options[:d.N])
		}
	}
}
