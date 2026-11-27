package kgen

// CopyPermanent Choices$ (Calamity, Galloping Inferno): the controller's
// mandatory one-pick over a battlefield filter, then the copy of the pick
// (gorge effects/copypermanent.go's supportsChoice arm: DefinedSpec "Valid
// <filter>" is the pool, one KChoose with Min 1 and Max 1 posed even for one
// candidate and none for no candidate). Lowered to OpChooseCard and
// ExtCopyPicked over RefChosen; with nothing to pick gorge copies nothing but
// still registers the AtEOT$ rider's delayed trigger (scheduleAtEOTNone),
// which ExtCopyPicked does too.

// copyChoicePick lowers CopyPermanent's Choices$ pick and returns the ref the
// copy reads. Every parameter the picked-copy op does not carry refuses by
// name.
func (lc *lowerCtx) copyChoicePick(ps *params) (lRef, error) {
	// Pawprint$, RandomCopied$, RandomNum$ and ValidSupportedCopy$ block gorge's
	// CopyPermanent altogether (compileCopyPermanent: it mints nothing); Chooser$,
	// Controller$ and WithDifferentNames$ leave its single-pick arm (supportsChoice)
	for _, k := range []string{"ValidTgts", "Defined", "Chooser", "WithDifferentNames", "Populate", "NumCopies",
		"WithCountersType", "PumpKeywords", "RememberTokens", "AtEOTTrig", "Controller", "DefinedName",
		"Pawprint", "RandomCopied", "RandomNum", "ValidSupportedCopy"} {
		if _, ok := ps.m[k]; ok {
			return lRef{}, refuse("CopyPermanent Choices$ beside %s$", k)
		}
	}
	f, err := lc.filter(ps.str("Choices"))
	if err != nil {
		return lRef{}, err
	}
	ps.consume("Choices", "ChoiceTitle")
	lc.emit(lInstr{Op: "ChooseCard", Obj: lRef{Kind: rYou}, F: f, C: zBattlefield, CS: "CopyPick"})
	return lRef{Kind: rChosen}, nil
}

// copyChoiceAttacking reads TokenAttacking$ beside Choices$: True only, in
// an attack trigger of the controller's own attack (tokenAttacking's gate).
// The copy's defender is gorge's players-only CR 508.4 fallback and ask
// (copyAttackRider: a copy has no planeswalker/battle slot).
func (lc *lowerCtx) copyChoiceAttacking(ps *params) (bool, error) {
	if v, ok := ps.get("TokenAttacking"); ok && v != "True" {
		return false, refuse("CopyPermanent Choices$ with TokenAttacking$ %q", v)
	}
	return lc.copyAttacking(ps)
}

// copyAttacking reads CopyPermanent's TokenAttacking$ for the target/copy
// paths (Flamerush Rider, Shaun, Father of Synths), which raise the copy's
// CopyAttacking flag (instr_copy.go opCopyAsToken). Only literal True in an
// attack trigger of the controller's own attack is modelled: the copy's
// defender is gorge's players-only CR 508.4 fallback and ask
// (copyAttackRider). A selector names its own defender, which the copy op
// does not carry. A no-trigger program refuses too: tokenAttacking lets it
// through for the Token op (the kernel gates that mint on the combat window
// at run time), but the copy entry has no such gate, and gorge's
// enteringDefenderFallback enters a token attacking only inside combat.
func (lc *lowerCtx) copyAttacking(ps *params) (bool, error) {
	v, ok := ps.get("TokenAttacking")
	if !ok {
		return false, nil
	}
	if v != "True" {
		return false, refuse("CopyPermanent TokenAttacking$ %q", v)
	}
	if _, err := lc.tokenAttacking(v); err != nil {
		return false, err
	}
	if lc.ab.Trigger == nil {
		return false, refuse("CopyPermanent TokenAttacking$ True outside an attack trigger")
	}
	return true, nil
}
