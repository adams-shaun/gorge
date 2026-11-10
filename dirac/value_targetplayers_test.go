package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTargetPlayersExpr: ExTargetPlayers reads 1 when its target slot holds a
// player and 0 for an unfilled slot, a permanent target and an unnamed slot,
// reads each slot on its own, and on the resolving entry follows the
// resolution's illegal-target mask (a player that stopped being a legal
// target no longer counts, CR 608.2b).
func TestTargetPlayersExpr(t *testing.T) {
	sc, id := counterKindsScen(t)
	s, k := sc.s, sc.k
	bears := sc.perm(id("Grizzly Bears"), 1)
	read := func(ref kop.Ref, e *stackEntry) int {
		x := kop.Expr{Kind: kop.ExTargetPlayers, Ref: ref}
		return s.exprCold(k, &x, evalCtx{ctrl: 0, src: NoObj, srcSl: NoSlot, e: e})
	}
	e := stackEntry{nt: 2}
	for i := range e.tgt {
		e.tgt[i] = NoTarget
	}
	e.tgt[1] = PlayerTarget0 + 1
	if got := read(kop.RefTarget0, &e); got != 0 {
		t.Errorf("an unfilled slot reads %d, want 0", got)
	}
	if got := read(kop.RefTarget1, &e); got != 1 {
		t.Errorf("a slot holding the opponent reads %d, want 1", got)
	}
	e.tgt[0] = uint8(bears)
	if got := read(kop.RefTarget0, &e); got != 0 {
		t.Errorf("a permanent target reads %d, want 0", got)
	}
	if got := read(kop.RefController, &e); got != 0 {
		t.Errorf("a ref that is no target slot reads %d, want 0", got)
	}
	if x := (kop.Expr{Kind: kop.ExTargetPlayers, Ref: kop.RefTarget1}); s.exprCold(k, &x, evalCtx{ctrl: 0, src: NoObj, srcSl: NoSlot}) != 0 {
		t.Error("an evaluation with no stack entry must read 0")
	}
	s.res.e = e
	s.res.bad = 0
	if got := read(kop.RefTarget1, &s.res.e); got != 1 {
		t.Errorf("the resolving entry's player slot reads %d, want 1", got)
	}
	s.res.bad = 1 << 1
	if got := read(kop.RefTarget1, &s.res.e); got != 0 {
		t.Errorf("the resolving entry's illegal slot reads %d, want 0", got)
	}
	s.res.bad = 0
}
