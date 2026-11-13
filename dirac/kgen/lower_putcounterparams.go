package kgen

import "strings"

// PutCounter's Choices$ (a mid-resolution recipient
// pick: Aerith Rescue Mission, Wick, the Whorled Mind) and RememberPut$
// (Systems Override).

// notePutRememberPut marks a PutCounter whose recipient joins the
// resolution's remembered set (RememberPut$; B=2).
const notePutRememberPut = "remember put"

// putCounterChoices lowers PutCounter with Choices$ (gorge counters_put.go
// putCounterChoose -> putCounterChooseTitled): the resolving controller picks
// one battlefield permanent the Choices$ spec admits, scanned seat 0's
// battlefield then seat 1's; none: nothing; one: taken unasked; two or more:
// one KChoose (Min 1, Max 1, counter_pick). The pick gets CounterNum$
// counters through the ordinary CounterChange. ChoiceTitle$ is the prompt
// only. Every shape beside the bare Min 1 / Max 1 controller pick refuses.
func (lc *lowerCtx) putCounterChoices(ps *params) error {
	for _, k := range []string{"ValidTgts", "MinChoiceAmount", "ChoiceAmount", "Chooser", "ForColor",
		"CounterTypes", "RememberPut", "RememberAmount", "CounterNumPerDefined", "Optional"} {
		if _, ok := ps.m[k]; ok {
			return refuse("PutCounter Choices$ with %s$", k)
		}
	}
	// Defined$ is unread on the pick path: gorge's putCounterChooseTitled
	// takes the recipient from the pick alone, and lists Defined$ in
	// putCounterKnownKeys as deliberately ignored (Forge's CountersPutEffect
	// Choices$ branch never reads it either), so the script's referrer names
	// the pick's outcome for its reader only (Gluntch, the Bestower's
	// "They put two +1/+1 counters on a creature they control" beside its
	// Choices$; Haphazard Bombardment's Defined$ ChosenCard). Gorge Notes
	// nothing for it either, so consuming it unread matches gorge decision
	// for decision.
	ps.consume("Defined")
	spec := ps.str("Choices")
	for _, a := range strings.Split(spec, ",") {
		if strings.Contains(a, "inZone") && !strings.Contains(a, "inZoneBattlefield") {
			return refuse("PutCounter Choices$ %q: not a battlefield filter", spec) // gorge scans the battlefield only
		}
	}
	ct := ps.str("CounterType")
	if ct == "" {
		return refuse("PutCounter without CounterType")
	}
	if strings.Contains(ct, ",") {
		return refuse("PutCounter Choices$ with a CounterType$ list")
	}
	if ct == "ENERGY" {
		return refuse("PutCounter Choices$ ENERGY")
	}
	f, err := lc.filter(spec)
	if err != nil {
		return err
	}
	n, err := lc.amount(ps, "CounterNum", 1)
	if err != nil {
		return err
	}
	put := lInstr{Op: "PutCounter", Obj: lRef{Kind: rChosen}, A: n, CS: ct}
	if v, ok := ps.get("RememberCards"); ok {
		// RememberCards$ names the recipient that took the counters (gorge
		// rememberPlaced): the same B=2 arm RememberPut$ uses, remembered
		// when the CounterNum$ amount is > 0.
		if v != "True" {
			return refuse("PutCounter Choices$ with RememberCards$ %q", v)
		}
		put.B, put.Note = cst(2), notePutRememberPut
		ps.consume("RememberCards")
	}
	ps.consume("Choices", "ChoiceTitle")
	lc.emit(lInstr{Op: "ChooseCard", Obj: lRef{Kind: rYou}, F: f, C: zBattlefield, CS: "PutCounterPick"})
	lc.emit(put)
	return nil
}

// putCounterRememberPut lowers RememberPut$ True (gorge counters_put.go's
// Defined$/target loop, rememberPlaced): the recipient joins the
// resolution's remembered set when the CounterNum$ amount it was given is
// > 0, read before counter replacements (gorge does not ask that a counter
// landed). A Defined$ filter that excludes the target places nothing and
// remembers nothing. B=2 carries it.
func putCounterRememberPut(ps *params, in *lInstr) error {
	v, ok := ps.get("RememberPut")
	if !ok {
		return nil
	}
	if v != "True" {
		return refuse("RememberPut %q", v)
	}
	if _, ok := ps.m["RememberAmount"]; ok {
		return refuse("PutCounter RememberAmount$ with RememberPut$")
	}
	if in.Obj.Kind != rSelf && in.Obj.Kind != rTarget {
		return refuse("PutCounter RememberPut$ on %v", in.Obj)
	}
	in.B = cst(2)
	in.Note = notePutRememberPut
	ps.consume("RememberPut")
	return nil
}
