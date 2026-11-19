package kgen

import "fmt"

// A reflexive trigger's body reading the parent resolution's remembered set.
//
// gorge effImmediateTrigger (engine/effects/immediate.go) hands every instance
// of an ImmediateTrigger WITHOUT RememberObjects$ the parent chain's
// capture-excluded remembered set (wholeSet = parent when remember == ""), so
// the body's Remembered$CardManaCost or Remembered$Amount reads what the
// parent remembered: Calibrated Blast and Yuna's Whistle's "deals damage /
// puts counters equal to that card's mana value", the card a DigUntil
// RememberFound$ revealed. The kernel's latent starts with an empty set
// (resolve.go), which refuseCrossAbilityRemembered refuses. The set is fixed
// once the instance is put on the stack and a card's mana value does not
// change with its zone, so the read is captured at the parent exactly as
// TriggerRemembered$ is (lower_trigremembered.go): the body reads
// vTriggerAmount and OpReflexiveTrigger's B is the parent-context value.
//
// Fail-closed: the body writes no remembered set of its own, every
// Remembered read of its program is one such value (one rest for the
// latent: there is one trigAmt), its targets and conditions read none, and a
// ClearRemembered is its last instruction only.

// remReflexParentSet rewrites lat's body reads (see above) and sets its
// TrigRem; it leaves the latent untouched when the shape does not hold.
func (lc *lowerCtx) remReflexParentSet(ps *params, lat *lAbility) {
	if _, ok := ps.m["RememberObjects"]; ok || lat.TrigRem != "" || len(lat.Prog) == 0 {
		return
	}
	for _, t := range lat.Targets {
		if kremReads(fmt.Sprint(t.F)) {
			return
		}
	}
	if kremReads(fmt.Sprint(lat.ActCond)) {
		return
	}
	rest := ""
	prog := make([]lInstr, len(lat.Prog))
	for i, in := range lat.Prog {
		if kremWrites(in) || in.Op == "ClearRemembered" && i != len(lat.Prog)-1 {
			return
		}
		var ok bool
		if in.A, ok = remReflexValue(in.A, &rest); !ok {
			return
		}
		if in.B, ok = remReflexValue(in.B, &rest); !ok {
			return
		}
		if kremReads(in.String()) {
			return // a remembered read other than the two captured values
		}
		prog[i] = in
	}
	if rest == "" {
		return
	}
	lat.Prog, lat.TrigRem = prog, rest
}

// remReflexValue replaces each Remembered$CardManaCost (MVOf the remembered
// set, summed) and Remembered$Amount (RememberedCount ctx) in v by the
// captured trigger amount, recording the rest in *rest; a second, different
// rest is not one trigAmt and fails.
func remReflexValue(v lValue, rest *string) (lValue, bool) {
	r := ""
	switch {
	case v.Kind == vMVOf && v.Ref.Kind == rRemembered && v.N == 0:
		r = "CardManaCost"
	case v.Kind == vRememberedCount && v.S == "ctx":
		r = "Amount"
	}
	if r != "" {
		if *rest != "" && *rest != r {
			return v, false
		}
		*rest = r
		v.Kind, v.Ref, v.S = vTriggerAmount, lRef{}, ""
		return v, true
	}
	if len(v.Sub) == 0 {
		return v, true
	}
	sub := make([]lValue, len(v.Sub))
	for i, s := range v.Sub {
		var ok bool
		if sub[i], ok = remReflexValue(s, rest); !ok {
			return v, false
		}
	}
	v.Sub = sub
	return v, true
}
