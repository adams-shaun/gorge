package kgen

import (
	"github.com/adams-shaun/gorge/engine/cards"
)

// The api: effects for counters and the mechanics that read them (lower_api.go).
func init() {
	apiOp("PutCounter", (*lowerCtx).apiPutCounter)
	apiOp("RemoveCounter", (*lowerCtx).apiRemoveCounter)
	apiOp("MultiplyCounter", (*lowerCtx).apiMultiplyCounter)
	apiOp("AddOrRemoveCounter", (*lowerCtx).apiAddOrRemoveCounter) // lower_addorremovecounter.go
	apiPS("Endure", (*lowerCtx).endure)                            // lower_endure.go
	apiPS("PutCounterAll", (*lowerCtx).putCounterAll)
	apiPS("RemoveCounterAll", (*lowerCtx).removeCounterAll)
	apiPS("Amass", (*lowerCtx).amass)
	apiPS("Empower", (*lowerCtx).empower) // lower_empower.go
	apiPS("Proliferate", (*lowerCtx).proliferate)
	apiPS("MoveCounter", (*lowerCtx).moveCounter)
	apiPS("CumulativeUpkeep", (*lowerCtx).cumulativeUpkeep) // lower_cumulative.go
	apiPS("Echo", (*lowerCtx).echoUpkeep)                   // lower_echo.go
}

func (lc *lowerCtx) apiPutCounter(sa *cards.SA, ps *params) error {
	if done, err := lc.definedValid(sa, ps); done || err != nil {
		return err
	}
	if ps.is("Mentor", "True") {
		return lc.mentorPutCounter(ps) // lower_mentor.go
	}
	return lc.putCounter(ps)
}

func (lc *lowerCtx) apiRemoveCounter(sa *cards.SA, ps *params) error {
	if _, ok := ps.m["Choices"]; ok {
		return lc.rmctrChoices(ps) // lower_removecounterchoices.go
	}
	obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	if ps.is("CounterType", "Any") && !ps.is("CounterNum", "All") {
		// gorge removeCounterAny (cr-removecounter-any-upto): a kind
		// pick per counter, UpTo$ a Done at every step (Glissa
		// Sunslayer)
		n, err := lc.amount(ps, "CounterNum", 1)
		if err != nil {
			return err
		}
		if !n.isConst() || n.N < 1 || n.N > 8 {
			return refuse("RemoveCounter Any CounterNum %v", n)
		}
		in := lInstr{Op: "RemoveCounterAny", Obj: obj, B: n}
		if v, ok := ps.get("UpTo"); ok {
			if v != "True" {
				return refuse("RemoveCounter UpTo$ %q", v)
			}
			in.C = 1
		}
		if ps.is("RememberRemoved", "True") {
			return refuse("RemoveCounter Any RememberRemoved$")
		}
		lc.emit(in)
		return nil
	}
	var n lValue
	if ps.is("CounterNum", "All") {
		if ps.is("CounterType", "All") {
			return lc.clearCounters(ps, obj)
		}
		// gorge effRemoveCounter's NumAll: every counter of the kind the
		// object holds, read before the removal; a CounterType$ naming
		// no single kind (Any) refuses
		if k := ps.str("CounterType"); k == "" || k == "Any" {
			return refuse("RemoveCounter All of CounterType %q", ps.str("CounterType"))
		}
		n = lValue{Kind: vCountersOn, Ref: obj, S: ps.str("CounterType")}
	} else if n, err = lc.amount(ps, "CounterNum", 1); err != nil {
		return err
	}
	in := lInstr{Op: "RemoveCounter", Obj: obj, A: n, CS: ps.str("CounterType")}
	if ps.is("RememberRemoved", "True") {
		in.Note = "remember removed"
		in.C = 1
	}
	lc.emit(in)
	return nil
}

func (lc *lowerCtx) apiMultiplyCounter(sa *cards.SA, ps *params) error {
	if m := ps.str("Multiplier"); m != "" && m != "2" { // gorge effMultiplyCounter's default is 2
		return refuse("MultiplyCounter Multiplier$ %q", m)
	}
	if done, err := lc.definedValid(sa, ps); done || err != nil {
		return err
	}
	obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	in := lInstr{Op: "DoubleCounters", Obj: obj, CS: ps.str("CounterType")}
	if in.CS == "" {
		in.CS = "All"
	}
	lc.emit(in)
	return nil
}
