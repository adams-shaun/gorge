package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCondAnd pins kop.CondAnd's truth table: both Sub conditions must
// hold, re-evaluated on every read (CR 611.3a), and Neg inverts the
// conjunction as a whole (not either operand).
func TestCondAnd(t *testing.T) {
	set := cardsettest.Set(t, "condand")
	tbl0, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	tbl := *tbl0
	tbl.Conds = append([]kop.Cond(nil), tbl.Conds...)
	cond := func(c kop.Cond) kop.CondID {
		tbl.Conds = append(tbl.Conds, c)
		return kop.CondID(len(tbl.Conds) - 1)
	}
	yours := cond(kop.Cond{Kind: kop.CondYourTurn})
	yes := cond(kop.Cond{Kind: kop.CondValue, Op: kop.CmpGE, A: kop.Const(1), B: kop.Const(0)})
	no := cond(kop.Cond{Kind: kop.CondValue, Op: kop.CmpGE, A: kop.Const(0), B: kop.Const(1)})
	and := func(a, b kop.CondID, neg bool) kop.CondID {
		return cond(kop.Cond{Kind: kop.CondAnd, Sub: [2]kop.CondID{a, b}, Neg: neg})
	}
	tt, tf := and(yours, yes, false), and(yours, no, false)
	ft, ff := and(no, yes, false), and(no, no, false)
	notTT := and(yours, yes, true)

	lib, ok := set.Lookup("Plains")
	if !ok {
		t.Fatal("Plains not in the pool")
	}
	sc := newScenTableLib(t, &tbl, CardID(set.PoolID(lib)), Config{})
	if sc.s.active != 0 {
		t.Fatalf("scenario active player %d, want 0", sc.s.active)
	}
	eval := func(id kop.CondID, ctrl uint8) bool {
		return sc.s.condEval(sc.k, id, evalCtx{ctrl: ctrl})
	}
	for _, c := range []struct {
		name string
		id   kop.CondID
		ctrl uint8
		want bool
	}{
		{"your turn (controller)", yours, 0, true},
		{"your turn (opponent)", yours, 1, false},
		{"value true", yes, 0, true},
		{"value false", no, 0, false},
		{"true and true", tt, 0, true},
		{"true and true, opponent's turn", tt, 1, false},
		{"true and false", tf, 0, false},
		{"false and true", ft, 0, false},
		{"false and false", ff, 0, false},
		{"negated conjunction", notTT, 0, false},
		{"negated conjunction, opponent's turn", notTT, 1, true},
	} {
		if got := eval(c.id, c.ctrl); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
