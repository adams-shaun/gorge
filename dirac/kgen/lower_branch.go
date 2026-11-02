package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// branch lowers api:Branch (gorge effBranch): the BranchConditionSVar$
// value compared (BranchConditionSVarCompare$, default GE1) once; true runs
// TrueSubAbility$, false FalseSubAbility$. Lowered as
//
//	SkipIf cond (skip T+1); T...; SkipIf never (skip F); F...
//
// so the condition is read once, before either arm changes it.
func (lc *lowerCtx) branch(ps *params) error {
	if _, ok := ps.get("ValidTgts"); ok {
		// the branch's own target (the arms' Defined$ Targeted*)
		if _, err := lc.objOrTarget(ps, "", zNone); err != nil {
			return err
		}
	}
	sv := ps.str("BranchConditionSVar")
	if sv == "" {
		return refuse("Branch without BranchConditionSVar$")
	}
	v, err := lc.value(sv)
	if err != nil {
		return err
	}
	cmp := ps.str("BranchConditionSVarCompare")
	if cmp == "" {
		cmp = "GE1"
	}
	op, b, err := lc.cmpSpec(cmp)
	if err != nil {
		return refuse("Branch compare %q", cmp)
	}
	arm := func(key string) (*cards.SA, error) {
		name := ps.str(key)
		if name == "" {
			return nil, nil
		}
		sa := cards.ResolveSVar(lc.face.SVars, name)
		if sa == nil {
			return nil, refuse("Branch %s$ %q", key, name)
		}
		return sa, nil
	}
	t, err := arm("TrueSubAbility")
	if err != nil {
		return err
	}
	f, err := arm("FalseSubAbility")
	if err != nil {
		return err
	}
	at := len(lc.ab.Prog)
	lc.emit(lInstr{Op: "SkipIf", Cond: lCond{Kind: cValue, A: v, Op: op, B: b}})
	if t != nil {
		if err := lc.chain(t, nil); err != nil {
			return err
		}
	}
	if f == nil {
		lc.ab.Prog[at].Skip = len(lc.ab.Prog) - at - 1
		return nil
	}
	lc.ab.Prog[at].Skip = len(lc.ab.Prog) - at // over T and the jump
	jt := len(lc.ab.Prog)
	lc.emit(lInstr{Op: "SkipIf", Cond: lCond{Kind: cValue, A: cst(0), Op: opGE, B: cst(1)}, Note: "jump over the false arm"})
	if err := lc.chain(f, nil); err != nil {
		return err
	}
	lc.ab.Prog[jt].Skip = len(lc.ab.Prog) - jt - 1
	return nil
}

// compareValue lowers Count$Compare <SVar> <op><n>.<then>.<else> (gorge
// evalCompare) when the compared SVar is a battlefield count
// (Count$Valid <filter>) and the test is GE or GT a literal: Dazzling
// Denial's "{4} if you control a Bird, else {2}".
func (lc *lowerCtx) compareValue(arg string, depth int) (lValue, error) {
	name, rest, _ := strings.Cut(arg, " ")
	if len(rest) < 3 {
		return lValue{}, refuse("Count$Compare %q", arg)
	}
	op, tail := rest[:2], rest[2:]
	thTok, branches, _ := strings.Cut(tail, ".")
	ifTok, elseTok, ok := strings.Cut(branches, ".")
	th, err := strconv.Atoi(thTok)
	if !ok || err != nil || strings.Contains(elseTok, ".") {
		return lValue{}, refuse("Count$Compare %q", arg)
	}
	cop, ok := opNamesCmp[op]
	if !ok || cop == opNE {
		// gorge evalCompare reads GE GT EQ LE LT only
		return lValue{}, refuse("Count$Compare operator %q", op)
	}
	body, ok := lc.face.SVars[name]
	if !ok {
		body, ok = compareInlineBody(name) // value_compareheads.go
	}
	if !ok {
		return lValue{}, refuse("Count$Compare %s names no SVar", name)
	}
	collected := strings.TrimSpace(body) == "Collected$Amount"
	var cv lValue
	if collected {
		if err := lc.collectedCompare(op, th); err != nil {
			return lValue{}, err // lower_collectedevidence.go
		}
	} else if cv, err = lc.svarValue(body, depth+1); err != nil {
		return lValue{}, err
	}
	var v lValue
	if collected {
		v = lValue{Kind: vKicked} // paid: the then arm (Kicked.<paid>.<unpaid>)
	} else if (op == "GE" || op == "GT") && cv.Kind == vCount && cv.F != nil && cv.Mul == 0 && cv.Div == 0 && cv.Add == 0 && !cv.Neg {
		if op == "GT" {
			th++
		}
		v = lValue{Kind: vCountAtLeast, F: cv.F, N: th}
	} else {
		// any other compared value or operator: ExCompare
		v = lValue{Kind: vCompare, N: th, Op: cop, Sub: []lValue{cv, {Kind: vBranches}}}
	}
	arms := &v
	if v.Kind == vCompare {
		arms = &v.Sub[1]
	}
	for _, t := range []string{ifTok, elseTok} {
		if n, err := strconv.Atoi(t); err == nil {
			arms.Sub = append(arms.Sub, cst(n))
			continue
		}
		sb, ok := lc.face.SVars[t]
		if !ok {
			return lValue{}, refuse("Count$Compare branch %q", t)
		}
		sv, err := lc.svarValue(sb, depth+1)
		if err != nil {
			return lValue{}, err
		}
		arms.Sub = append(arms.Sub, sv)
	}
	return v, nil
}
