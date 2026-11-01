package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDiscover: OpDiscover exiles from the top until a nonland card with
// mana value <= A (lands and bigger cards go to the bottom, in order) and
// remembers it; the tail puts it into hand while it is still exiled.
func TestDiscover(t *testing.T) {
	deck := []cardset.Entry{{Name: "Savannah Lions", Count: 2}, {Name: "Plains", Count: 18}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	lc := CardID(255)
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Savannah Lions" {
			lc = CardID(i)
		}
	}
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Discover", Types: kop.TSorcery, Abilities: []kop.Ability{
		{Kind: kop.AbSpell, Prog: []kop.Instr{{Op: kop.OpDiscover, Obj: kop.RefController, A: kop.Const(1)}}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	pl := &s.pl[0]
	lion := sc.obj(lc, 0)
	s.objs[lion].zone = kop.ZoneLibrary
	pl.libTop(lion)
	// order from the top: a Plains, then the Lions
	pl.lib[pl.libLo], pl.lib[pl.libLo+1] = pl.lib[pl.libLo+1], pl.lib[pl.libLo]
	top1 := pl.lib[pl.libLo]
	s.res.e.ctrl = 0
	s.exec(sc.k, &kop.Instr{Op: kop.OpDiscover, Obj: kop.RefController, A: kop.Const(1)})
	if s.objs[lion].zone != kop.ZoneExile || s.g.nrem != 1 || s.g.rem[0] != lion {
		t.Fatalf("Lions zone %d, remembered %d, want exiled and remembered", s.objs[lion].zone, s.g.nrem)
	}
	if s.objs[top1].zone != kop.ZoneLibrary || pl.lib[pl.libHi-1] != top1 {
		t.Fatal("the exiled Plains is not on the library bottom")
	}
	s.exec(sc.k, &kop.Instr{Op: kop.OpDiscover, C: 1})
	if s.objs[lion].zone != kop.ZoneHand {
		t.Fatalf("the tail: Lions zone %d, want hand", s.objs[lion].zone)
	}
}
