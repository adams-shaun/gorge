package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRearrange: OpRearrange asks the controller to order every looked-at
// card (no Done; the first pick on top), then with C&1 (MayShuffle$) a
// yes/no shuffle ask (gorge arrange_mayshuffle).
func TestRearrange(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	ra := kop.Instr{Op: kop.OpRearrange, Obj: kop.RefController, A: kop.Const(3), C: 1}
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Ponder", Types: kop.TSorcery, Abilities: []kop.Ability{
		{Kind: kop.AbSpell, Prog: []kop.Instr{ra}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	pl := &s.pl[0]
	top := [3]Obj{pl.lib[pl.libLo], pl.lib[pl.libLo+1], pl.lib[pl.libLo+2]}
	s.res.e.ctrl, s.res.e.card, s.res.e.ab, s.res.pc = 0, src, 0, 0
	if s.exec(sc.k, &ra) || s.res.ask != raGenRearrange {
		t.Fatalf("ask %d, want the rearrange ask", s.res.ask)
	}
	for _, want := range []int{3, 2, 1} {
		var d Decision
		s.resolveOptionsGen(sc.k, &d)
		if d.Sub != ChArrange || int(d.N) != want {
			t.Fatalf("sub %d with %d options, want ChArrange over %d", d.Sub, d.N, want)
		}
		s.applyAskCold2(sc.k, d.Options[d.N-1]) // the last offered card next from the top
	}
	if s.res.ask != raGenRearrShuffle {
		t.Fatalf("ask %d after the picks, want the shuffle ask", s.res.ask)
	}
	if pl.lib[pl.libLo] != top[2] || pl.lib[pl.libLo+1] != top[1] || pl.lib[pl.libLo+2] != top[0] {
		t.Fatal("the library top is not the picked order")
	}
	var d Decision
	s.resolveOptionsGen(sc.k, &d)
	if d.Sub != ChGenConfirm || d.N != 2 || d.Options[0].Mode() != 1 {
		t.Fatalf("shuffle ask: sub %d with %d options", d.Sub, d.N)
	}
	s.applyAskCold2(sc.k, d.Options[1]) // no: the order stays
	if pl.lib[pl.libLo] != top[2] {
		t.Fatal("declined: the library changed")
	}
}
