package kgen

import "strings"

// the cast provenance of a permanent as it enters, read
// from its own resolving spell (kernel PredWasCastEnter, cast_wascastenter.go).
// A trigger's ValidCard(s)$ wasCast / !wasCast / wasCastFromGraveyard on a
// battlefield entry lowers through the filter word (filter.go); the enters
// replacement's CheckSVar$ Count$wasCastFromYourHandByYou.1.0 (Patched
// Plaything, Myojin of Night's Reach) lowers here to the same predicate on
// the entering object, CmpN wasCastHand.

// wasCastHandCond lowers a self ETB replacement's CheckSVar$ when it names
// Count$wasCastFromYourHandByYou.1.0. ok is false for any other CheckSVar$
// (the caller reads it as a value). An arm other than 1.0, an SVarCompare$,
// or another object's replacement refuses (only "if you cast it from
// your hand" is the exact read).
func (lc *lowerCtx) wasCastHandCond(rp *params, other bool) (c lCond, ok bool, err error) {
	cs, has := rp.get("CheckSVar")
	if !has {
		return lCond{}, false, nil
	}
	sv := cs
	if !strings.HasPrefix(sv, "Count$") {
		sv = lc.face.SVars[cs]
	}
	arms, isHand := strings.CutPrefix(sv, "Count$wasCastFromYourHandByYou.")
	if !isHand {
		return lCond{}, false, nil
	}
	if other || lc.cloneRepl {
		return lCond{}, false, refuse("Count$wasCastFromYourHandByYou in another object's enters replacement")
	}
	if arms != "1.0" {
		return lCond{}, false, refuse("Count$wasCastFromYourHandByYou arms %q", arms)
	}
	if _, cmp := rp.m["SVarCompare"]; cmp {
		return lCond{}, false, refuse("Count$wasCastFromYourHandByYou beside SVarCompare$")
	}
	f := &lFilter{Clauses: []lClause{{Flags: fSelf, CmpStat: cmpWasCast, CmpN: wasCastHand}}}
	return lCond{Kind: cRefMatches, Ref: lRef{Kind: rSelf}, F: f, Op: opGE, A: cst(1)}, true, nil
}

// filterReadsSpellProv: some clause of f reads a spell's cast provenance
// (cmpWasCastSpell): the filter is matched against a spell
// on the stack, not a permanent. A mana ability's spend restriction is one
// (grantedManaShape keeps only that restriction).
func filterReadsSpellProv(f *lFilter) bool {
	if f == nil {
		return false
	}
	for _, c := range f.Clauses {
		if c.CmpStat == cmpWasCastSpell {
			return true
		}
	}
	return false
}
