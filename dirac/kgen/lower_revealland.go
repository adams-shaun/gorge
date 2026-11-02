package kgen

import "strings"

// revealLandCost reads a reveal land's UnlessCost$ Reveal<1/Spec[/description]>
// (or Theorist's Sanctum's Behold<1/Spec>) and returns the verb with the spec,
// its ";" alternation folded to "," (gorge cost parse). Every carrier in the
// corpus takes one card.
func revealLandCost(uc string) (verb, spec string, ok bool) {
	verb = "Reveal"
	rest, ok := strings.CutPrefix(strings.TrimSpace(uc), "Reveal<1/")
	if !ok {
		verb = "Behold"
		rest, ok = strings.CutPrefix(strings.TrimSpace(uc), "Behold<1/")
	}
	if !ok {
		return "", "", false
	}
	rest, ok = strings.CutSuffix(rest, ">")
	if !ok {
		return "", "", false
	}
	spec, _, _ = strings.Cut(rest, "/")
	return verb, strings.ReplaceAll(spec, ";", ","), spec != ""
}

// revealLandElection lowers "As this enters, you may reveal a <spec> card from
// your hand. If you don't, it enters tapped" to UnlessPaid(Reveal, You) over
// EnterTapped; a behold (Theorist's Sanctum) is UnlessPaid(Behold, You).
func (lc *lowerCtx) revealLandElection(verb, spec string) error {
	f, err := lc.filter(spec)
	if err != nil {
		return err
	}
	if f.Players != 0 || len(f.Clauses) == 0 {
		return refuse("ETB tap UnlessCost Reveal filter %s", f)
	}
	lc.emit(lInstr{Op: "UnlessPaid", CS: verb, A: cst(1), F: f, Obj: lRef{Kind: rYou}, Skip: 1})
	lc.emit(lInstr{Op: "EnterTapped", Obj: lRef{Kind: rReplaced}})
	return nil
}
