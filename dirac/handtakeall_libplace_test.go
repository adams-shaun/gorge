package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// handTakeAllLibraryScen builds a table with one mandatory OpChooseMove that
// takes two cards from the controller's hand to the top or bottom of the
// controller's library, plus the card that holds it.
func handTakeAllLibraryScen(t *testing.T, bottom bool) (*scen, CardID, CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 40}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Zone: kop.ZoneHand})
	f := kop.FilterID(len(tbl.Filters) - 1)
	b := 0
	if bottom {
		b = kop.SrHandBottom
	}
	mv := kop.Instr{Op: kop.OpChooseMove, Obj: kop.RefController, A: kop.Const(2), F: f,
		C: int16(kop.ZoneLibrary) | int16(kop.SrMandatory)<<3, B: kop.Const(b)}
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

// startHandTakeAll runs the scenario's mandatory take-all and returns the
// picked cards and the posed arrangement's option order.
func startHandTakeAll(t *testing.T, bottom bool) (sc *scen, first, second Obj, opts []Obj) {
	t.Helper()
	sc, src, pl := handTakeAllLibraryScen(t, bottom)
	s := sc.s
	first, second = sc.hand(pl, 0), sc.hand(pl, 0)
	in := sc.k.t.Cards[src].Abilities[0].Prog[0]
	s.res = resCtx{e: stackEntry{kind: seAbility, obj: NoObj, card: src, ab: 0, ctrl: 0, trigObj: NoObj}, active: true, lastTok: NoObj, eachObj: NoObj}
	started := s.startSearch(sc.k, &in)
	if s.res.ask != raGenLibPlace {
		t.Fatalf("ask %d, want the library placement ask after the no-choice take-all", s.res.ask)
	}
	if started {
		// gorge's handLibraryTail asks the arrangement mid-resolution (the
		// spell still on the stack), so startSearch parks the resolution at
		// the ask; the answered arrangement continues the program
		// (applyAskCold2's raGenLibPlace).
		t.Fatal("the take-all reported the search done with the arrangement ask pending")
	}
	if s.res.askP != 0 {
		t.Fatalf("asked seat %d, want the cards' owner 0", s.res.askP)
	}
	var d Decision
	s.resolveOptionsGen(sc.k, &d)
	if d.Sub != ChArrange || int(d.N) != 2 {
		t.Fatalf("decision sub %d with %d options, want ChArrange over both moved cards", d.Sub, d.N)
	}
	for i := 0; i < int(d.N); i++ {
		opts = append(opts, Obj(d.Options[i].Source()))
	}
	if s.objs[first].zone != kop.ZoneLibrary || s.objs[second].zone != kop.ZoneLibrary {
		t.Fatalf("zones %d/%d, want both cards in the library", s.objs[first].zone, s.objs[second].zone)
	}
	return sc, first, second, opts
}

// TestHandTakeAllLibraryAsksPlacementOrder pins the dirac half of gorge's
// CR 401.4 fix: a required hand move that takes every eligible card into one
// library position (Sawtooth Loon's "put two cards from your hand on the
// bottom of your library" with exactly two in hand) poses the owner's
// arrangement, although no card pick is asked. The answer is the placement
// order: the first pick nearest the top of the bottom group.
func TestHandTakeAllLibraryAsksPlacementOrder(t *testing.T) {
	sc, first, second, opts := startHandTakeAll(t, true)
	s := sc.s
	if len(opts) != 2 || opts[0] != first || opts[1] != second {
		t.Fatalf("options %v, want the move order [%d %d]", opts, first, second)
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

// TestHandTakeAllLibraryTopAsksPlacementOrder is the top-position half: the
// take-all puts the cards on top as it moves (reverse move order), and the
// ask must still offer them in gorge's move order, then apply the answer
// nearest the top.
func TestHandTakeAllLibraryTopAsksPlacementOrder(t *testing.T) {
	sc, first, second, opts := startHandTakeAll(t, false)
	s := sc.s
	if len(opts) != 2 || opts[0] != first || opts[1] != second {
		t.Fatalf("options %v, want the move order [%d %d]", opts, first, second)
	}
	s.applyAskCold2(sc.k, MakeAction(AkChoose, TCode(second), 0, 0))
	s.applyAskCold2(sc.k, MakeAction(AkChoose, TCode(first), 0, 0))
	if got := libTopOrder(s, 0, 2); got[0] != second || got[1] != first {
		t.Fatalf("library top %v, want the first pick %d then %d", got, second, first)
	}
}
