package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestJoinForcesAccumulator: ExtJoinForces adds each B to the resolution's
// total and ExJoinForces reads it (3 then 4 read 7); B = 0 resets it; and
// scrResEnter, the start of the next resolution, returns it to 0.
func TestJoinForcesAccumulator(t *testing.T) {
	tbl := *v.Table
	tbl.Exprs = append(append([]kop.Expr(nil), tbl.Exprs...), kop.Expr{Kind: kop.ExJoinForces})
	read := kop.ValueRef{K: kop.VExpr, N: int16(len(tbl.Exprs) - 1)}
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	sc.s.res = resCtx{e: stackEntry{ctrl: 0}, active: true}
	sc.s.scrResEnter()

	add := func(n int) {
		t.Helper()
		in := kop.Instr{Op: kop.OpExt, A: kop.Const(int(kop.ExtJoinForces)), B: kop.Const(n)}
		if _, done := sc.s.execExt(sc.k, &in); !done {
			t.Fatal("ext dispatch did not claim ExtJoinForces")
		}
	}
	got := func() int { return sc.s.evalV(sc.k, read, evalCtx{}) }

	if g := got(); g != 0 {
		t.Fatalf("total at the start of a resolution = %d, want 0", g)
	}
	add(3)
	if g := got(); g != 3 {
		t.Fatalf("after adding 3: %d, want 3", g)
	}
	add(4)
	if g := got(); g != 7 {
		t.Fatalf("after adding 3 then 4: %d, want 7", g)
	}
	add(0)
	if g := got(); g != 0 {
		t.Fatalf("after the reset (B = 0): %d, want 0", g)
	}
	add(5)
	if g := got(); g != 5 {
		t.Fatalf("after reset and 5: %d, want 5", g)
	}
	sc.s.scrResEnter()
	if g := got(); g != 0 {
		t.Fatalf("after scrResEnter: %d, want 0 (the total is per resolution)", g)
	}
	add(32767)
	add(10)
	if g := got(); g != 32767 {
		t.Fatalf("past the ceiling: %d, want it saturated at 32767", g)
	}
}
