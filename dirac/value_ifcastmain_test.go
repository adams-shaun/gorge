package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestIfCastInOwnMain: ExIfCastOwnMain reads three live conjuncts (a main
// step, the controller the active player, the source spell cast), so a spell
// cast in a main phase and resolved in another step reads the no arm, as does
// a stack copy, another player's turn, and a non-spell entry.
func TestIfCastInOwnMain(t *testing.T) {
	sc, _ := newBLBScen(t, []cardset.Entry{{Name: "Plains", Count: 8}})
	x := &sc.k.t.Exprs
	*x = append(*x, kop.Expr{Kind: kop.ExIfCastOwnMain, Sub: [2]kop.ValueRef{kop.Const(2), kop.Const(1)}})
	v := kop.ValueRef{K: kop.VExpr, N: int16(len(*x) - 1)}
	const o Obj = 5
	s := sc.s
	cast := &stackEntry{kind: seSpell, obj: o, ctrl: 0}
	eval := func(step kop.Step, active uint8, e *stackEntry, of uint8) int {
		s.step, s.active = step, active
		s.objs[o].of = 0
		if of != 0 {
			s.objs[o].of = of
		}
		return s.evalV(sc.k, v, evalCtx{e: e, ctrl: 0, src: o})
	}
	if got := eval(kop.StepMain1, 0, cast, 0); got != 2 {
		t.Fatalf("precondition: a cast spell in its controller's first main phase read %d, want the yes arm 2", got)
	}
	for _, tc := range []struct {
		name   string
		step   kop.Step
		active uint8
		e      *stackEntry
		of     uint8
		want   int
	}{
		{"second main", kop.StepMain2, 0, cast, 0, 2},
		{"combat step", kop.StepAttackers, 0, cast, 0, 1},
		{"end step", kop.StepEnd, 0, cast, 0, 1},
		{"the opponent's main phase", kop.StepMain1, 1, cast, 0, 1},
		{"a stack copy", kop.StepMain1, 0, cast, ofCopy, 1},
		{"an ability entry", kop.StepMain1, 0, &stackEntry{kind: seTrigger, obj: o}, 0, 1},
		{"no entry", kop.StepMain1, 0, nil, 0, 1},
	} {
		if got := eval(tc.step, tc.active, tc.e, tc.of); got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, got, tc.want)
		}
	}
	s.objs[o].of = 0
}
