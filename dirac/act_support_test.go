package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSupport: OpSupport asks the controller for up to A creatures (other
// than the source when C&0x100), one +1/+1 counter each, closing at the
// A-th pick or Done; nothing eligible: no ask.
func TestSupport(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	bear := CardID(len(tbl.Cards))
	sup := []kop.Instr{{Op: kop.OpSupport, A: kop.Const(2), C: int16(kop.CtrP1P1) | 0x100}}
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s, k := sc.s, sc.k
	src, b1, b2, b3 := sc.perm(bear, 0), sc.perm(bear, 0), sc.perm(bear, 1), sc.perm(bear, 1)
	s.res.e.obj, s.res.e.ctrl = src, 0
	if s.support(k, &sup[0]) {
		t.Fatal("eligible creatures: support must ask")
	}
	var d Decision
	s.supportOptions(k, &sup[0], &d)
	if d.N != 4 { // b1, b2, b3 and Done; never the source
		t.Fatalf("%d options, want 4", d.N)
	}
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkChoose && Obj(a.Source()) == src {
			t.Fatal("the source is offered")
		}
	}
	_ = b3
	// apply the picks by hand (answerSupport needs the resolving program)
	s.res.peekN, s.res.left = 0, 2
	for _, o := range []Obj{b1, b2} {
		s.res.arr[s.res.peekN] = o
		s.res.peekN++
		s.bolsterPut(k, s.objs[o].slot, int(kop.CtrP1P1), 1)
		s.res.left--
	}
	if s.perms[s.objs[b1].slot].ctrLow(int(kop.CtrP1P1)) != 1 || s.perms[s.objs[b2].slot].ctrLow(int(kop.CtrP1P1)) != 1 {
		t.Fatal("each pick gets one +1/+1 counter")
	}
	var d2 Decision
	s.supportOptions(k, &sup[0], &d2)
	if d2.N != 2 { // b3 and Done
		t.Fatalf("%d options after two picks, want 2", d2.N)
	}
	none := []kop.Instr{{Op: kop.OpSupport, A: kop.Const(0), C: int16(kop.CtrP1P1)}}
	if !s.support(k, &none[0]) {
		t.Fatal("Support 0 does nothing")
	}
}
