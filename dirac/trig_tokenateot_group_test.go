package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTokenAtEOTSacCombatGroups pins the one-trigger group of an AtEOT$
// SacrificeCombat resolution that mints several tokens: gorge's
// scheduleAtEOTGroup registers one delayed triggered ability naming every
// token it made (CR 603.7), so the end of combat puts one trigger on the
// stack, not one per token.
func TestTokenAtEOTSacCombatGroups(t *testing.T) {
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
	sc.s.res = resCtx{e: stackEntry{kind: seSpell, ctrl: 0, trigObj: NoObj}, active: true}
	head := sc.s.newToken(tok, 0)
	second := sc.s.newToken(tok, 0)
	in := kop.Instr{Op: kop.OpCreateTokenEx,
		B: kop.Const(kop.TokSacAtEnd | kop.TokExileAtEnd | int(kop.TokAtEOTSacCombat)<<8)}

	n0 := sc.s.ndly
	sc.s.g.tokMade = 1
	sc.s.tokenAtEOTExt(&in, head)
	sc.s.g.tokMade = 2
	sc.s.tokenAtEOTExt(&in, second)
	if sc.s.ndly != n0+2 {
		t.Fatalf("delayed records %d -> %d, want head and one follower", n0, sc.s.ndly)
	}
	if d := sc.s.dly[n0]; d.when != dlyEndCombat || d.obj != head {
		t.Fatalf("head record = %+v, want the end-of-combat sacrifice of %d", d, head)
	}
	if d := sc.s.dly[n0+1]; d.when != dlyGrpFollow || d.creator != head || d.obj != second {
		t.Fatalf("follower record = %+v, want a follower of %d", d, head)
	}
	np := sc.s.npend
	sc.s.fireDlyEndCombat(nil)
	if got := sc.s.npend - np; got != 1 {
		t.Fatalf("end of combat queued %d triggers, want one group trigger", got)
	}
	if sc.s.ndly != n0+1 {
		t.Fatalf("delayed records after the fire = %d, want the follower kept for the group", sc.s.ndly)
	}
}
