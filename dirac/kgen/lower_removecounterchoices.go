package kgen

// noteRmctrRemember marks a Support instr in its remove form
// that also carries RememberAmount$ True.
const noteRmctrRemember = notePutRememberAmount

// rmctrMaxPicks is the pick cap the kernel's Support ask shares with
// proliferate (prolifMaxPicks).
const rmctrMaxPicks = 16

// rmctrChoices lowers RemoveCounter with Choices$ (gorge removeCounterChoose):
// the resolving controller elects permanents among every battlefield
// permanent matching Choices$ and each loses one counter of CounterType$.
// ChoiceNum$ N (no ChoiceOptional$) is an exact N; ChoiceOptional$ True with
// no ChoiceNum$ is any number. Emits Support C bits 2 (remove form), 4
// (exact), 8 (RememberAmount$ True: the removed total joins the remembered
// count). Anything else refuses by name.
func (lc *lowerCtx) rmctrChoices(ps *params) error {
	kind := ps.str("CounterType")
	if kind == "" || kind == "Any" || kind == "All" {
		return refuse("RemoveCounter Choices$ with CounterType$ %q", kind)
	}
	for _, k := range []string{"Defined", "ValidTgts", "ChoiceZone", "UpTo", "CounterNumShared", "RememberRemoved"} {
		if _, ok := ps.m[k]; ok {
			return refuse("RemoveCounter Choices$ with %s$", k)
		}
	}
	if ps.is("CounterNum", "All") {
		return refuse("RemoveCounter Choices$ with CounterNum$ All")
	}
	cn, err := lc.amount(ps, "CounterNum", 1)
	if err != nil {
		return err
	}
	if !cn.isConst() || cn.N != 1 {
		return refuse("RemoveCounter Choices$ with CounterNum$ %v", cn)
	}
	f, err := lc.filter(ps.str("Choices"))
	if err != nil {
		return err
	}
	if f.Players != 0 {
		return refuse("RemoveCounter Choices$ with a player clause")
	}
	optional := false
	if v, ok := ps.get("ChoiceOptional"); ok {
		if v != "True" {
			return refuse("RemoveCounter ChoiceOptional$ %q", v)
		}
		optional = true
	}
	_, numSet := ps.m["ChoiceNum"]
	n := cst(1)
	if numSet {
		if optional {
			return refuse("RemoveCounter ChoiceOptional$ with ChoiceNum$")
		}
		if n, err = lc.amount(ps, "ChoiceNum", 1); err != nil {
			return err
		}
		if !n.isConst() || n.N < 1 || n.N > rmctrMaxPicks {
			return refuse("RemoveCounter ChoiceNum$ %v", n)
		}
	}
	c := 2
	if optional {
		n = cst(rmctrMaxPicks)
	} else {
		c |= 4
	}
	in := lInstr{Op: "Support", A: n, F: f, CS: kind, C: c}
	if v, ok := ps.get("RememberAmount"); ok {
		if v != "True" {
			return refuse("RemoveCounter RememberAmount %q", v)
		}
		in.C |= 8
		in.Note = noteRmctrRemember
	}
	lc.emit(in)
	return nil
}

// isRememberAmountWriter: a PutCounter or a remove-form Support whose
// RememberAmount$ adds only the count (remCnt) to the remembered set.
func isRememberAmountWriter(in lInstr) bool {
	return (in.Op == "PutCounter" || in.Op == "Support") && in.Note == notePutRememberAmount
}
