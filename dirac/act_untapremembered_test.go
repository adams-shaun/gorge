package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestUntapRemembered: OpUntap with Obj$ Remembered must untap every
// remembered object, not only rem[0] (gorge effUntap walks the whole
// Defined$ set; Hunting Wilds kicked untaps both fetched Forests). Tokens
// remembered by one OpCreateTokenEx are the carrier, tapped by hand; a
// single remembered object stays on the core's one-object path.
func TestUntapRemembered(t *testing.T) {
	// untapOnRemembered creates n tapped tokens remembered by one
	// OpCreateTokenEx, runs one OpUntap over RefRemembered, and returns
	// whether each remembered token is still tapped, in remember order.
	untapOnRemembered := func(t *testing.T, tokens int) []bool {
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
		for i := 0; i < tokens; i++ {
			sc.s.perms[sc.s.objs[sc.s.g.rem[i]].slot].flags |= pfTapped
		}
		untap := kop.Instr{Op: kop.OpUntap, Obj: kop.RefRemembered}
		sc.s.exec(sc.k, &untap)
		got := make([]bool, tokens)
		for i := range got {
			got[i] = sc.s.perms[sc.s.objs[sc.s.g.rem[i]].slot].flags&pfTapped != 0
		}
		return got
	}

	got := untapOnRemembered(t, 2)
	t.Logf("still tapped: rem[0]=%v rem[1]=%v", got[0], got[1])
	if got[0] || got[1] {
		t.Fatalf("still tapped: rem[0]=%v rem[1]=%v, want both untapped (gorge effUntap loops Defined$ Remembered)",
			got[0], got[1])
	}

	// The nrem == 1 boundary: one remembered token takes the core's arm.
	if one := untapOnRemembered(t, 1); one[0] {
		t.Fatal("single remembered token still tapped")
	}
}
