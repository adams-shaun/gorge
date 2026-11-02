package kgen

import "strings"

// revealLandCost reads a reveal land's UnlessCost$ Reveal<1/Spec[/description]>
// and returns the spec with its ";" alternation folded to "," (gorge cost
// parse). Every carrier in the corpus reveals one card.
func revealLandCost(uc string) (string, bool) {
	rest, ok := strings.CutPrefix(strings.TrimSpace(uc), "Reveal<1/")
	if !ok {
		return "", false
	}
	rest, ok = strings.CutSuffix(rest, ">")
	if !ok {
		return "", false
	}
	spec, _, _ := strings.Cut(rest, "/")
	return strings.ReplaceAll(spec, ";", ","), spec != ""
}

// revealLandElection lowers "As this enters, you may reveal a <spec> card from
// your hand. If you don't, it enters tapped" to UnlessPaid(Reveal, You) over
// EnterTapped.
func (lc *lowerCtx) revealLandElection(spec string) error {
	f, err := lc.filter(spec)
	if err != nil {
		return err
	}
	if f.Players != 0 || len(f.Clauses) == 0 {
		return refuse("ETB tap UnlessCost Reveal filter %s", f)
	}
	lc.emit(lInstr{Op: "UnlessPaid", CS: "Reveal", A: cst(1), F: f, Obj: lRef{Kind: rYou}, Skip: 1})
	lc.emit(lInstr{Op: "EnterTapped", Obj: lRef{Kind: rReplaced}})
	return nil
}
