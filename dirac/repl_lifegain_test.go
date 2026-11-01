package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestLifeRepl: the controller's life-gain doublers double a gain, its
// plus-N ones add N (gorge applyLifeReplacements); an opponent's do not
// apply; a table holding both kinds stalls (StallLifeRepl).
func TestLifeRepl(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	mk := func(kinds ...kop.ReplKind) (*scen, []CardID) {
		tbl, _, _, err := set.Matchup(deck, deck)
		if err != nil {
			t.Fatal(err)
		}
		var ids []CardID
		for _, kd := range kinds {
			ids = append(ids, CardID(len(tbl.Cards)))
			tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Boon", Types: kop.TEnchantment, Abilities: []kop.Ability{
				{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kd, A: 1}}}})
		}
		return newScenTable(t, tbl, Config{NoElide: true}), ids
	}
	sc, ids := mk(kop.ReplLifeDouble)
	s := sc.s
	sc.perm(ids[0], 0)
	life0, life1 := s.pl[0].life, s.pl[1].life
	s.gainLife(sc.k, 0, 3)
	s.gainLife(sc.k, 1, 3)
	if s.pl[0].life-life0 != 6 || s.pl[1].life-life1 != 3 {
		t.Fatalf("doubler: gained %d and %d, want 6 and 3", s.pl[0].life-life0, s.pl[1].life-life1)
	}
	sc, ids = mk(kop.ReplLifePlus)
	s = sc.s
	sc.perm(ids[0], 0)
	sc.perm(ids[0], 0)
	life0 = s.pl[0].life
	s.gainLife(sc.k, 0, 3)
	if s.pl[0].life-life0 != 5 {
		t.Fatalf("two plus-1: gained %d, want 5", s.pl[0].life-life0)
	}
	sc, _ = mk(kop.ReplLifeDouble, kop.ReplLifePlus)
	if sc.s.g.stalled != StallLifeRepl {
		t.Fatalf("both kinds: stalled %d, want StallLifeRepl", sc.s.g.stalled)
	}
}
