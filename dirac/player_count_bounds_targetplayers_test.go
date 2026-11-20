package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTargetPlayersExprCountsEveryExtra: ExTargetPlayers counts the player
// targets a slot holds, not just its first: a multi-object slot over "any
// number of target opponents" holds three at four seats (Hollow Marauder).
// An extra pick of another slot or one that stopped being legal does not
// count (CR 608.2b).
func TestTargetPlayersExprCountsEveryExtra(t *testing.T) {
	sc, _ := counterKindsScen(t)
	s, k := sc.s, sc.k
	e := stackEntry{nt: 2}
	for i := range e.tgt {
		e.tgt[i] = TCode(NoTarget)
	}
	e.tgt[1] = TCode(PlayerTarget0 + 1)
	xt := s.xtMut(&e)
	xt.n = 3
	xt.slot[0], xt.code[0] = 1, TCode(PlayerTarget0+2)
	xt.slot[1], xt.code[1] = 1, TCode(PlayerTarget0+3)
	xt.slot[2], xt.code[2] = 0, TCode(PlayerTarget0+1) // another slot
	read := func(ref kop.Ref, e *stackEntry) int {
		x := kop.Expr{Kind: kop.ExTargetPlayers, Ref: ref}
		return s.exprCold(k, &x, evalCtx{ctrl: 0, src: NoObj, srcSl: NoSlot, e: e})
	}
	if got := read(kop.RefTarget1, &e); got != 3 {
		t.Errorf("slot 1 holds three player targets, reads %d", got)
	}
	if got := read(kop.RefTarget0, &e); got != 1 {
		t.Errorf("slot 0 holds one player extra, reads %d", got)
	}
	s.res.e = e
	s.res.bad = 0
	s.res.xbad = 1 << 1 // the second extra stopped being legal
	if got := read(kop.RefTarget1, &s.res.e); got != 2 {
		t.Errorf("an illegal extra target counts, reads %d, want 2", got)
	}
	s.res.xbad = 0
}
