package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestConnive: OpConnive draws, then asks the discard when the hand holds
// more than the count (raDiscard); OpConniveCounters then puts a +1/+1
// counter per nonland card discarded. A whole-hand discard of lands gives
// none.
func TestConnive(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	bear, spell := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1)
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Rogue", Types: kop.TCreature, Power: 1, Toughness: 1},
		kop.CardDef{Name: "Test Spell", Types: kop.TInstant})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	r := sc.perm(bear, 0)
	sc.s.res.e.ctrl, sc.s.res.e.obj, sc.s.res.e.card, sc.s.res.e.srcVer = 0, r, bear, sc.s.objs[r].ver
	cv := kop.Instr{Op: kop.OpConnive, Obj: kop.RefSelf, A: kop.Const(1)}
	cc := kop.Instr{Op: kop.OpConniveCounters, Obj: kop.RefSelf}
	// an empty hand: draw a Plains, discard it (no ask), no counter
	sc.s.exec(sc.k, &cv)
	sc.s.exec(sc.k, &cc)
	if sc.s.pl[0].handN != 0 || sc.s.perms[sc.slot(r)].ctrLow(int(kop.CtrP1P1)) != 0 {
		t.Fatalf("hand %d, counters %d after conniving a land, want 0 and 0", sc.s.pl[0].handN, sc.s.perms[sc.slot(r)].ctrLow(int(kop.CtrP1P1)))
	}
	// a nonland in hand: the discard is asked; discarding it gives a counter
	x := sc.hand(spell, 0)
	if sc.s.exec(sc.k, &cv) || sc.s.res.ask != raDiscard {
		t.Fatalf("ask %d, want the discard ask over two cards", sc.s.res.ask)
	}
	sc.s.res.ask = raNone
	sc.s.discardObj(sc.k, x)
	sc.s.exec(sc.k, &cc)
	if n := sc.s.perms[sc.slot(r)].ctrLow(int(kop.CtrP1P1)); n != 1 {
		t.Fatalf("%d +1/+1 counters after discarding a nonland, want 1", n)
	}
}
