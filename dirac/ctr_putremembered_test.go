package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPutCounterRemembered: OpPutCounter with Obj$ Remembered must place the
// counters on every remembered object, not only rem[0]. gorge's
// effPutCounter walks the whole Defined$ set (engine/effects/counters_put.go:240;
// Defined$ Remembered is the resolution's whole remembered set,
// engine/effects/context.go:634). Two tokens remembered by one
// OpCreateTokenEx (TokRememberObj) are the measured carrier (Recursive
// Recruitment); a single remembered object stays on the core's one-object path.
func TestPutCounterRemembered(t *testing.T) {
	// putOnRemembered creates n tokens remembered by one OpCreateTokenEx,
	// then runs one OpPutCounter over RefRemembered. It returns the +1/+1
	// count of each remembered token in remember order.
	putOnRemembered := func(t *testing.T, tokens int) []int {
		t.Helper()
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
		sc.k.t.Exprs = append(sc.k.t.Exprs, kop.Expr{Kind: kop.ExConst, N: int16(tokens)})
		n := kop.ValueRef{K: kop.VExpr, N: int16(len(sc.k.t.Exprs) - 1)}
		sc.s.res.e.ctrl = 0
		sc.s.g.nrem = 0
		create := kop.Instr{Op: kop.OpCreateTokenEx, Obj: kop.RefController, A: n, C: int16(tok),
			B: kop.Const(kop.TokRemember | kop.TokRememberObj)}
		sc.s.exec(sc.k, &create)
		if int(sc.s.g.nrem) != tokens {
			t.Fatalf("remembered %d tokens, want %d", sc.s.g.nrem, tokens)
		}
		put := kop.Instr{Op: kop.OpPutCounter, Obj: kop.RefRemembered, A: kop.Const(2), C: int16(kop.CtrP1P1)}
		sc.s.exec(sc.k, &put)
		got := make([]int, tokens)
		for i := range got {
			got[i] = sc.s.perms[sc.s.objs[sc.s.g.rem[i]].slot].ctrLow(int(kop.CtrP1P1))
		}
		return got
	}

	got := putOnRemembered(t, 2)
	t.Logf("counters: rem[0]=%d rem[1]=%d", got[0], got[1])
	if got[0] != 2 || got[1] != 2 {
		t.Fatalf("counters: rem[0]=%d rem[1]=%d, want 2 and 2 (gorge effPutCounter loops Defined$ Remembered)",
			got[0], got[1])
	}

	// The nrem == 1 boundary: the same program with one remembered token
	// still takes the core's single-object OpPutCounter arm.
	one := putOnRemembered(t, 1)
	if one[0] != 2 {
		t.Fatalf("single remembered counter = %d, want 2", one[0])
	}
}
