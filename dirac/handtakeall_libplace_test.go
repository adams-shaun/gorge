package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// handTakeAllLibraryScen builds a table with one mandatory OpChooseMove that
// takes two cards from the controller's hand to the bottom of the controller's
// library, plus the card that holds it.
func handTakeAllLibraryScen(t *testing.T) (*scen, CardID, CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 40}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Zone: kop.ZoneHand})
	f := kop.FilterID(len(tbl.Filters) - 1)
	mv := kop.Instr{Op: kop.OpChooseMove, Obj: kop.RefController, A: kop.Const(2), F: f,
		C: int16(kop.ZoneLibrary) | int16(kop.SrMandatory)<<3, B: kop.Const(kop.SrHandBottom)}
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Take All", Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{mv}}}})
	pl := CardID(0)
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Plains" {
			pl = CardID(i)
		}
	}
	return newScenTable(t, tbl, Config{NoElide: true}), src, pl
}

// TestHandTakeAllLibraryAsksPlacementOrder pins the dirac half of gorge's
// CR 401.4 fix: a required hand move that takes every eligible card into one
// library position (Sawtooth Loon's "put two cards from your hand on the
// bottom of your library" with exactly two in hand) poses the owner's
// arrangement, although no card pick is asked. The answer is the placement
// order: the first pick nearest the top of the bottom group.
func TestHandTakeAllLibraryAsksPlacementOrder(t *testing.T) {
	sc, src, pl := handTakeAllLibraryScen(t)
	s := sc.s
	first, second := sc.hand(pl, 0), sc.hand(pl, 0)
	in := sc.k.t.Cards[src].Abilities[0].Prog[0]
	s.res = resCtx{e: stackEntry{kind: seAbility, obj: NoObj, card: src, ab: 0, ctrl: 0, trigObj: NoObj}, active: true, lastTok: NoObj, eachObj: NoObj}
	started := s.startSearch(sc.k, &in)
	if s.res.ask != raGenLibPlace {
		t.Fatalf("ask %d, want the library placement ask after the no-choice take-all", s.res.ask)
	}
	if !started {
		t.Fatal("the take-all did not report the move done")
	}
	if s.res.askP != 0 {
		t.Fatalf("asked seat %d, want the cards' owner 0", s.res.askP)
	}
	var d Decision
	s.resolveOptionsGen(sc.k, &d)
	if d.Sub != ChArrange || int(d.N) != 2 {
		t.Fatalf("decision sub %d with %d options, want ChArrange over both moved cards", d.Sub, d.N)
	}
	if s.objs[first].zone != kop.ZoneLibrary || s.objs[second].zone != kop.ZoneLibrary {
		t.Fatalf("zones %d/%d, want both cards in the library", s.objs[first].zone, s.objs[second].zone)
	}
	// Answer with the second card first: it ends nearest the top of the
	// bottom group.
	s.applyAskCold2(sc.k, MakeAction(AkChoose, TCode(second), 0, 0))
	s.applyAskCold2(sc.k, MakeAction(AkChoose, TCode(first), 0, 0))
	if s.res.ask == raGenLibPlace {
		t.Fatal("the arrangement is still posed after every card was ordered")
	}
	p := &s.pl[0]
	if got := p.lib[int(p.libHi)-2]; got != second {
		t.Fatalf("bottom group first card %d, want the first pick %d nearest the top", got, second)
	}
	if got := p.lib[int(p.libHi)-1]; got != first {
		t.Fatalf("bottom group last card %d, want the second pick %d", got, first)
	}
}
