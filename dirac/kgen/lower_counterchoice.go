package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// PutCounter whose CounterType$ lists counter kinds to choose from:
// "put your choice of a menace, trample, reach, or haste counter on target
// Dinosaur" (Owen Grady, Raptor Trainer), "enters with your choice of a
// deathtouch counter or a lifelink counter" (Boot Nipper). gorge (effects
// counters_put.go: the len(counterKinds) > 1 arm, after CounterType$ is split
// by splitCounterKinds into canonical kinds) reads the comma list as a choice,
// NOT as every kind (that is CounterTypes$, lower_counterlist.go and
// putCounterKinds): the ability's controller is asked one Min=Max=1
// "counter_kind" KChoose over the listed kinds, in list order, whatever the
// number of recipients and before they are looked at, and one counter of the
// answered kind is put (CounterNum$ one). It is asked even when only one kind
// would be legal on the recipient; a list of two or more kinds is never
// answered unasked. The resolving form is an ordinary PutCounter-shaped
// instruction; the ETB$ True form is the body of an as-enters replacement
// (gorge excludes a comma kind from the entry fold, so it runs after the
// permanent is on the battlefield, like Grimdancer's ChooseDifferent$ pick).
// Both lower to OpExt ExtPutCounterChoice (dirac/ctr_choice.go) over up to
// four kinds. A fifth kind, a repeated or unknown kind, a special entry
// (containing '_'), a count other than one, or the remembered set as the
// recipient refuses by name.

// counterChoiceKinds validates a comma CounterType$ list and returns its
// kinds in printed order.
func counterChoiceKinds(list string) ([]string, error) {
	var kinds []string
	for _, k := range strings.Split(list, ",") {
		if k = strings.TrimSpace(k); k == "" {
			continue // gorge splitCounterKinds drops empty entries
		}
		if strings.Contains(k, "_") {
			return nil, refuse("PutCounter kind list special entry %q", k)
		}
		if _, err := counterKind(k); err != nil {
			return nil, refuse("PutCounter kind list entry %q: %v", k, err)
		}
		for _, o := range kinds {
			if strings.EqualFold(o, k) {
				return nil, refuse("PutCounter kind list repeats %q", k)
			}
		}
		kinds = append(kinds, k)
	}
	if len(kinds) < 2 {
		return nil, refuse("PutCounter kind list %q", list)
	}
	if len(kinds) > kop.PutCtrChoiceMax {
		return nil, refuse("PutCounter choice of %d counter kinds (at most %d)", len(kinds), kop.PutCtrChoiceMax)
	}
	return kinds, nil
}

// putCounterChoice lowers the resolving form: PutCounter | CounterType$ A,B,C
// on obj.
func (lc *lowerCtx) putCounterChoice(obj lRef, n lValue, list string) error {
	if obj.Kind == rRemembered {
		return refuse("PutCounter kind choice on the remembered set")
	}
	if !n.isConst() || n.N != 1 {
		return refuse("PutCounter kind choice CounterNum$ %v", n)
	}
	kinds, err := counterChoiceKinds(list)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "PutCounterChoice", Obj: obj, A: n, CS: strings.Join(kinds, ","), Note: "your choice of counter kind"})
	return nil
}

// etbCounterChoice lowers the as-enters form: an ETB replacement's PutCounter
// | ETB$ True | CounterType$ A,B,C on the entering permanent. Only the lone
// instruction of an unchained self-enters program lowers. The replacement's own
// condition (Tizerus Charger's "escapes") gates the whole program before the
// ask, as it does an EnterWithCounters body.
func (lc *lowerCtx) etbCounterChoice(ps *params, node, sa *cards.SA, other bool) error {
	if other || node != sa || sa.Sub != nil || len(lc.ab.Prog) != 0 {
		return refuse("ETB PutCounter kind choice beside another ETB replacement part")
	}
	n, err := lc.amount(ps, "CounterNum", 1)
	if err != nil {
		return err
	}
	if !n.isConst() || n.N != 1 {
		return refuse("ETB PutCounter kind choice CounterNum$ %v", n)
	}
	kinds, err := counterChoiceKinds(ps.str("CounterType"))
	if err != nil {
		return err
	}
	ps.consume("SpellDescription")
	lc.emit(lInstr{Op: "PutCounterChoice", Obj: lRef{Kind: rReplaced}, A: n, CS: strings.Join(kinds, ","), Note: "choice of a counter kind as it enters"})
	return nil
}

// buildPutCounterChoice is OpExt ExtPutCounterChoice over the kinds in cs.
func buildPutCounterChoice(ki *kop.Instr, cs string) error {
	var kinds []kop.CounterKind
	for _, n := range strings.Split(cs, ",") {
		c, err := counterKind(n)
		if err != nil {
			return err
		}
		kinds = append(kinds, c)
	}
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtPutCounterChoice))
	ki.B, ki.C = kop.PutCtrChoiceOperands(kinds)
	return nil
}
