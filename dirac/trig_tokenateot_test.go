package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTokenAtEOT: OpCreateTokenEx with TokSacAtEnd / TokExileAtEnd
// registers one next-end-step delayed action per created token (gorge
// scheduleAtEOT), the copy-token rider's dly entry.
func TestTokenAtEOT(t *testing.T) {
	sc, _ := newBLBScen(t, []cardset.Entry{{Name: "Carrot Cake", Count: 1}, {Name: "Plains", Count: 8}})
	tok := CardID(0)
	for i := range sc.k.t.Cards {
		if sc.k.t.Cards[i].Types&kop.TToken != 0 {
			tok = CardID(i)
			break
		}
	}
	if tok == 0 {
		t.Fatal("no token in the BLB table")
	}
	sc.s.res = resCtx{e: stackEntry{kind: seSpell, ctrl: 0, trigObj: NoObj}, active: true, lastTok: NoObj, eachObj: NoObj}
	n0 := sc.s.ndly
	in := kop.Instr{Op: kop.OpCreateTokenEx, Obj: kop.RefController, A: kop.Const(2), C: int16(tok), B: kop.Const(kop.TokExileAtEnd)}
	sc.s.exec(sc.k, &in)
	if sc.s.ndly != n0+2 || kop.Op(sc.s.dly[n0].act) != kop.OpExile || sc.s.dly[n0+1].obj == sc.s.dly[n0].obj {
		t.Fatalf("delayed entries %d -> %d", n0, sc.s.ndly)
	}
}

// TestOpAtEOT: OpAtEOT registers a next-end-step sacrifice (C=0) or exile
// (C=1) of a battlefield permanent; nothing for an object off it.
func TestOpAtEOT(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 2}, {Name: "Plains", Count: 8}})
	fc := id["Valley Floodcaller"]
	a := sc.perm(fc, 0)
	sc.s.res.e.ctrl, sc.s.res.e.obj, sc.s.res.e.card = 0, a, fc
	n0 := sc.s.ndly
	in := kop.Instr{Op: kop.OpAtEOT, Obj: kop.RefSelf, C: 1}
	sc.s.exec(sc.k, &in)
	if sc.s.ndly != n0+1 || kop.Op(sc.s.dly[n0].act) != kop.OpExile || sc.s.dly[n0].obj != a {
		t.Fatalf("delayed entries %d -> %d", n0, sc.s.ndly)
	}
	sc.s.leave(sc.k, sc.slot(a), kop.ZoneGraveyard, false)
	sc.s.exec(sc.k, &in)
	if sc.s.ndly != n0+1 {
		t.Fatal("an object off the battlefield got a rider")
	}
}
