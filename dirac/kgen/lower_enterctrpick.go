package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The as-enters choice of two different counter kinds from three (Grimdancer):
// K:ETBReplacement:Other:<SVar> with DB$ PutCounter | ETB$ True | CounterType$
// A,B,C | ChooseDifferent$ 2. gorge (effects counters_put.go, the
// ChooseDifferent arm) asks the controller a Min=Max=2 "counter_kinds" KChoose
// over the listed kinds, in list order, and puts CounterNum$ (one) counters of
// each answered kind on the entering permanent. The kernel lowers the body to
// the enters program's only instruction, OpExt ExtEnterCtrPick over the three
// kinds (dirac/repl_enterctrpick.go: two sequential picks). Only this exact
// shape lowers: gorge's ask is a fixed two-of-the-list pick whatever
// ChooseDifferent$'s value, so another value, another list length, an amount
// beyond one counter, a chain or a condition refuse.

// etbCounterPick lowers an ETB replacement's PutCounter ChooseDifferent$ node.
func (lc *lowerCtx) etbCounterPick(ps *params, node, sa *cards.SA, other bool, cond *lCond) error {
	if other || node != sa || sa.Sub != nil || cond.Kind != cNone || len(lc.ab.Prog) != 0 {
		return refuse("ETB PutCounter ChooseDifferent$ beside another ETB replacement part")
	}
	if !ps.is("ETB", "True") {
		return refuse("ETB PutCounter ChooseDifferent$ without ETB$ True")
	}
	if d, ok := ps.get("Defined"); ok && strings.TrimSpace(d) != "Self" {
		return refuse("ETB PutCounter ChooseDifferent$ Defined$ %q", d)
	}
	if v := strings.TrimSpace(ps.str("ChooseDifferent")); v != "2" {
		return refuse("ETB PutCounter ChooseDifferent$ %q", v)
	}
	if v, ok := ps.get("CounterNum"); ok && strings.TrimSpace(v) != "1" {
		return refuse("ETB PutCounter ChooseDifferent$ CounterNum$ %q", v)
	}
	var kinds []string
	for _, k := range strings.Split(ps.str("CounterType"), ",") {
		kinds = append(kinds, strings.TrimSpace(k))
	}
	if len(kinds) != 3 {
		return refuse("ETB PutCounter ChooseDifferent$ over %d counter kinds", len(kinds))
	}
	for i, k := range kinds {
		if _, err := counterKind(k); err != nil || strings.Contains(k, "_") {
			return refuse("ETB PutCounter ChooseDifferent$ kind %q", k)
		}
		for _, o := range kinds[:i] {
			if strings.EqualFold(o, k) {
				return refuse("ETB PutCounter ChooseDifferent$ repeats kind %q", k)
			}
		}
	}
	ps.consume("ChooseDifferent", "SpellDescription", "ETB")
	lc.emit(lInstr{Op: "EnterCtrPick", Obj: lRef{Kind: rReplaced}, CS: strings.Join(kinds, ","), Note: "choice of two different counter kinds as it enters"})
	return nil
}

// buildEnterCtrPick is OpExt ExtEnterCtrPick over the kinds in cs.
func buildEnterCtrPick(ki *kop.Instr, cs string) error {
	var k [3]kop.CounterKind
	for i, n := range strings.Split(cs, ",") {
		c, err := counterKind(n)
		if err != nil {
			return err
		}
		k[i] = c
	}
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtEnterCtrPick))
	ki.B = kop.Const(int(k[0]))
	ki.C = int16(k[1]) | int16(k[2])<<8
	return nil
}
