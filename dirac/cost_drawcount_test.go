package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// drawCostNoPay runs a Draw<N/You> cost ask whose resolved count is n and
// reports whether the ask is decline only (pfNoPay).
func drawCostNoPay(t *testing.T, n int) bool {
	t.Helper()
	set := cardsettest.Set(t, "mbc")
	deck := []cardset.Entry{{Name: "Plains", Count: 40}}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	prog := []kop.Instr{{Op: kop.OpUnlessPaid, Obj: kop.RefTargetController, A: kop.Const(n), C: kop.PayDraw}}
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Draw Cost", Types: kop.TSorcery, Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: prog}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	tgt := sc.perm(da[0], 0)
	sc.s.res.e = stackEntry{kind: seSpell, obj: tgt, srcVer: sc.s.objs[tgt].ver, card: src, ctrl: 0, nt: 1,
		tgt: [kop.MaxTargets]TCode{TCode(tgt)}, tver: [kop.MaxTargets]uint8{sc.s.objs[tgt].ver}}
	sc.s.res.pc = 0
	if sc.s.exec(sc.k, &prog[0]) || sc.s.res.ask != raGenPay {
		t.Fatalf("precondition: no pay ask (ask %d)", sc.s.res.ask)
	}
	return sc.s.g.pickF&pfNoPay != 0
}

// A Draw<X/You> cost is payable at a zero count and at a positive one, and
// decline only at a negative one: gorge pay.DrawCostCountTrig fails closed on
// n < 0 while a zero count draws nothing (Horrid Shadowspinner at power 0 vs
// power < 0).
func TestDrawCostCountSign(t *testing.T) {
	for _, c := range []struct {
		n     int
		noPay bool
	}{{-1, true}, {0, false}, {2, false}} {
		if got := drawCostNoPay(t, c.n); got != c.noPay {
			t.Errorf("Draw<%d>: decline-only = %v, want %v", c.n, got, c.noPay)
		}
	}
}
