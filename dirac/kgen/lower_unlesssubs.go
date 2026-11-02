package kgen

// UnlessResolveSubs$ WhenPaid / WhenNotPaid: gorge's
// unlessSubsRun (engine/effects/unless.go) gates the node's SubAbility$
// chain on the unless outcome. The pay instruction's skip covers the node
// only, so `unless` records the gate and `chain` applies it once the chain
// level's last node is lowered (the subs ARE the rest of that chain level).

// usGate is one recorded UnlessResolveSubs$ gate.
type usGate struct {
	at      int  // the pay instruction's index
	nodeEnd int  // len(Prog) after the unless node (the first sub instruction)
	same    bool // the subs run on the side the body runs
	nTgt    int  // len(Targets) after the unless node
	guarded bool // the node sits behind a guard of its own (refused at apply)
}

// unlessSubsMode reads UnlessResolveSubs$: whether a gate applies, and
// whether the subs run when the cost was paid.
func unlessSubsMode(ps *params) (gate, whenPaid bool, err error) {
	v, ok := ps.get("UnlessResolveSubs")
	if !ok {
		return false, false, nil
	}
	switch v {
	case "Always":
		return false, false, nil
	case "WhenPaid":
		return true, true, nil
	case "WhenNotPaid":
		return true, false, nil
	}
	return false, false, refuse("UnlessResolveSubs %q", v)
}

// markUnlessGuarded flags every gate recorded since n: the node that holds
// them has a guard of its own (a skip over the node alone).
func (lc *lowerCtx) markUnlessGuarded(n int) {
	for i := n; i < len(lc.unlessSubs); i++ {
		lc.unlessSubs[i].guarded = true
	}
}

// applyUnlessSubs applies the gates recorded since base (innermost, i.e.
// latest, first) to the chain level that just finished, then drops them.
func (lc *lowerCtx) applyUnlessSubs(base int) error {
	recs := lc.unlessSubs[base:]
	lc.unlessSubs = lc.unlessSubs[:base]
	for i := len(recs) - 1; i >= 0; i-- {
		r := recs[i]
		if r.guarded {
			return refuse("UnlessResolveSubs inside a guarded node")
		}
		end := len(lc.ab.Prog)
		if end == r.nodeEnd {
			continue // no subs: nothing to gate
		}
		if len(lc.ab.Targets) != r.nTgt {
			return refuse("UnlessResolveSubs over a sub that declares a target slot")
		}
		if r.same {
			lc.ab.Prog[r.at].Skip = end - r.at - 1
			continue
		}
		// the body's branch jumps over the subs; the other lands on them
		s := end - r.nodeEnd
		jump := lInstr{Op: "SkipIf", Cond: lCond{Kind: cValue, A: cst(0), Op: opGE, B: cst(1)}, Skip: s,
			Note: "jump over the UnlessResolveSubs$ subs"}
		lc.ab.Prog = append(lc.ab.Prog, lInstr{})
		copy(lc.ab.Prog[r.nodeEnd+1:], lc.ab.Prog[r.nodeEnd:])
		lc.ab.Prog[r.nodeEnd] = jump
		lc.ab.Prog[r.at].Skip++
	}
	return nil
}
