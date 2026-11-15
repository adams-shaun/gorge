package kgen

// CopyPermanent Populate$ True (CR 701.36a; Rootborn Defenses, Trostani's
// Judgment): the controller chooses a creature token they control and
// creates a token that's a copy of it. gorge's Populate$ arm
// (effects/copypermanent.go) scans "Valid Creature.token+YouCtrl": no token
// copies nothing and asks nothing, a lone token is copied unasked, several
// are one KChoose (copypermanent_choice). Lowered to the pick
// (OpChooseCard C choosePopulatePick) and ExtCopyPicked over RefChosen. The
// arm is taken only with no Defined$/ValidTgts$/Choices$/DefinedName$, and
// the picked-copy op carries no copy rider, so every other CopyPermanent
// parameter refuses by name (copy exceptions, in copyPermanent).

// populatePick lowers a Populate$ node's pick and returns the ref the copy
// reads.
func (lc *lowerCtx) populatePick(ps *params) (lRef, error) {
	for _, k := range []string{"ValidTgts", "Defined", "Choices", "DefinedName", "Chooser", "NumCopies",
		"WithCountersType", "PumpKeywords", "RememberTokens", "AtEOT", "AtEOTTrig", "TokenTapped",
		"TokenAttacking", "Controller", "Pawprint", "RandomCopied", "RandomNum", "ValidSupportedCopy"} {
		if _, ok := ps.m[k]; ok {
			return lRef{}, refuse("CopyPermanent Populate$ beside %s$", k)
		}
	}
	f, err := lc.filter("Creature.token+YouCtrl")
	if err != nil {
		return lRef{}, err
	}
	ps.consume("Populate")
	lc.emit(lInstr{Op: "ChooseCard", Obj: lRef{Kind: rYou}, F: f, C: zBattlefield, CS: "PopulatePick"})
	return lRef{Kind: rChosen}, nil
}
