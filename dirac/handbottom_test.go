package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// handBottomScen builds a table with one OpChooseMove from the opponent's
// hand into the library, carrying bottom as kop.SrHandBottom when set, plus
// the card that holds it. The hand under the pick belongs to player 1.
func handBottomScen(t *testing.T, bottom bool) (*scen, CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
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
	mv := kop.Instr{Op: kop.OpChooseMove, Obj: kop.RefOpponent, A: kop.Const(1), F: f,
		C: int16(kop.ZoneLibrary) | int16(kop.SrConfirm|kop.SrRemember)<<3, B: kop.Const(b)}
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Bottom Mover", Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{mv}}}})
	return newScenTable(t, tbl, Config{NoElide: true}), src
}

// startHandBottom poses the Move's confirmation and answers yes, then returns
// the one-card hand pick's instruction.
func startHandBottom(t *testing.T, sc *scen, src CardID) kop.Instr {
	t.Helper()
	in := sc.k.t.Cards[src].Abilities[0].Prog[0]
	sc.s.res.e.ctrl, sc.s.res.e.card, sc.s.res.e.ab, sc.s.res.pc = 0, src, 0, 0
	if sc.s.startSearch(sc.k, &in) || sc.s.res.ask != raGenConfirm {
		t.Fatalf("ask %d, want the hand move's yes/no confirmation", sc.s.res.ask)
	}
	var d Decision
	sc.s.askOptionsCold1(sc.k, &d)
	if d.Sub != ChGenConfirm || d.N != 2 || d.Options[0].Mode() != 1 || d.Options[1].Mode() != 0 {
		t.Fatalf("confirmation: sub %d with %d options, want yes then no", d.Sub, d.N)
	}
	sc.s.res.ask = raNone
	if sc.s.searchPick(sc.k) || sc.s.res.ask != raGenSearch {
		t.Fatalf("after yes: ask %d, want the hand pick", sc.s.res.ask)
	}
	return in
}

// TestHandBottomMove: a hand move carrying SrHandBottom puts the picked card
// at the bottom of its owner's library and remembers it under SrRemember; while
// the confirmation is pending nothing moves and nothing is remembered. The
// control clears SrHandBottom and the same pick lands on top, so the bit is
// what places the card.
func TestHandBottomMove(t *testing.T) {
	sc, src := handBottomScen(t, true)
	pl := CardID(255)
	for i := range sc.k.t.Cards {
		if sc.k.t.Cards[i].Name == "Plains" {
			pl = CardID(i)
		}
	}
	o, filler := sc.hand(pl, 1), sc.hand(pl, 1)
	if sc.s.objs[o].zone != kop.ZoneHand || sc.s.objs[filler].zone != kop.ZoneHand {
		t.Fatal("the setup cards are not both in the hand")
	}
	if sc.s.pl[1].libLen() < 2 {
		t.Fatalf("the setup library holds %d cards, too few to tell top from bottom", sc.s.pl[1].libLen())
	}
	startHandBottom(t, sc, src)
	// the pending confirmation has moved and remembered nothing
	if sc.s.objs[o].zone != kop.ZoneHand || sc.s.objs[filler].zone != kop.ZoneHand || sc.s.g.nrem != 0 {
		t.Fatalf("pending confirmation: o zone %d filler zone %d remembered %d, want both in hand and none remembered",
			sc.s.objs[o].zone, sc.s.objs[filler].zone, sc.s.g.nrem)
	}
	sc.s.resArm().look[0], sc.s.g.nlook = o, 1
	sc.s.finishSearchNow(sc.k)
	p := &sc.s.pl[1]
	if sc.s.objs[o].zone != kop.ZoneLibrary {
		t.Fatalf("picked card zone %d, want the library", sc.s.objs[o].zone)
	}
	if p.lib[p.libHi-1] != o {
		t.Fatal("the picked card is not the library's bottom card")
	}
	if p.lib[p.libLo] == o {
		t.Fatal("the picked card is on top, not the bottom")
	}
	if sc.s.objs[filler].zone != kop.ZoneHand {
		t.Fatalf("the unpicked card zone %d, want it still in hand", sc.s.objs[filler].zone)
	}
	if sc.s.g.nrem != 1 || sc.s.g.rem[0] != o {
		t.Fatalf("remembered %v, want the moved card", sc.s.g.rem[:sc.s.g.nrem])
	}

	// mutation: the same pick without the bottom bit lands on top
	sc2, src2 := handBottomScen(t, false)
	o2 := sc2.hand(pl, 1)
	sc2.hand(pl, 1)
	startHandBottom(t, sc2, src2)
	sc2.s.resArm().look[0], sc2.s.g.nlook = o2, 1
	sc2.s.finishSearchNow(sc2.k)
	p2 := &sc2.s.pl[1]
	if p2.lib[p2.libLo] != o2 {
		t.Fatal("without SrHandBottom the picked card is not on top")
	}
}
