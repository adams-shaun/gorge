package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestGrantSearchKeepsEngineOrderForUnnamedColours: Great Divide Guide grants every
// land "{T}: Add one mana of any color", so a {G}{W} play is priced by the prefix
// search. Breeding Pool's {U} and {G} are two plain abilities (its land types), whose
// "choose a mana ability" options name no colour, so gorge's scriptOrderAnswers does
// not move the needed {G} ahead: the first answer taps it for {U}, and the Forest and
// the Guide pay {G}{W} while {U} floats. Ordering the needed colour first tapped it
// for {G} and floated {G} (CR 601.2g-h).
func TestGrantSearchKeepsEngineOrderForUnnamedColours(t *testing.T) {
	set := cardset.AllowUnverified(cardsettest.Set(t, "standard-meta"))
	deck := []cardset.Entry{{Name: "Great Divide Guide", Count: 1}, {Name: "Breeding Pool", Count: 1}, {Name: "Forest", Count: 1},
		{Name: "Earth King's Lieutenant", Count: 1}, {Name: "Island", Count: 16}}
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	for _, n := range []string{"Breeding Pool", "Forest", "Great Divide Guide"} {
		if _, ok := id[n]; !ok {
			t.Fatalf("%s is not in the table", n)
		}
		sc.perm(id[n], 0)
	}
	lt := sc.perm(id["Earth King's Lieutenant"], 0)
	s, k := sc.s, sc.k
	cost := kop.ManaCost{}
	cost.Pips[kop.G], cost.Pips[kop.W] = 1, 1
	var need [6]bool
	need[kop.G], need[kop.W] = true, true
	var g grantSrch
	if !s.grantSearch(k, 0, &cost, sc.slot(lt), payDesc{}, need, &g) {
		t.Fatal("the search found no payment")
	}
	if g.np != 3 || g.path[0].col != kop.U {
		t.Fatalf("found %d steps, the first colour %d: want 3 steps, Breeding Pool's {U} first (engine order)", g.np, g.path[0].col)
	}
	s.grantApply(k, &g)
	if p := s.pl[0].pool; p[kop.U] != 1 || p[kop.G] != 1 || p[kop.W] != 1 || poolTotal(&s.pl[0]) != 3 {
		t.Errorf("pool %v, want U G W", p)
	}
}
