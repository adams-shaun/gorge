package kgen

import "strings"

// lower_tutorpick.go: "target opponent chooses one of the cards you
// searched for" (Intuition, Gifts Ungiven). The searcher's own library is the
// pool (DefinedPlayer$ You), the cards are the ones the preceding search
// remembered (ChangeType$ Card.IsRemembered), and the pick is answered by a
// targeted opponent (ValidTgts$ Opponent, Chooser$ Targeted):
//
//	DB$ ChangeZone | ValidTgts$ Opponent | DefinedPlayer$ You |
//	 Chooser$ Targeted | ChangeType$ Card.IsRemembered | Hidden$ True |
//	 Origin$ Library | Destination$ Hand | ChangeNum$ 1 | Mandatory$ True
//
// gorge's searchChooser/hiddenPickChooser resolve Chooser$ Targeted to the
// targeted player while DefinedPlayer$ fixes whose library is walked. The
// kernel poses that as the Chooser$ Opponent search (SrChooserOpp: the
// controller's first living opponent answers), which is the targeted
// opponent in a two-seat game, so the card is two-seat only: targetChooserPick
// marks it at the re-spell itself, since libSearchOwner (lower_zone.go) reads
// the genuine Chooser$ Opponent spelling, whose first-living-opponent answer
// gorge and the kernel do agree on past two seats. The target slot is still
// announced, as gorge announces it.

// targetChooserPick claims the opponent-chooses pick's player target and
// re-spells its Chooser$ as the kernel's opponent chooser. It reports false,
// with nothing read, for any other ChangeZone.
func (lc *lowerCtx) targetChooserPick(ps *params, origin int) (bool, error) {
	if origin != zLibrary || ps.peek("ValidTgts") != "Opponent" || ps.peek("DefinedPlayer") != "You" ||
		ps.peek("Chooser") != "Targeted" {
		return false, nil
	}
	for _, k := range []string{"TargetMin", "TargetMax", "OriginAlternative", "ChooseFromDefined", "RememberSearcher", "TargetUnique", "Optional"} {
		if _, ok := ps.m[k]; ok {
			return false, nil
		}
	}
	if lc.lastTarget() >= 0 {
		return false, refuse("an opponent-chooses library pick after another target slot")
	}
	pf, err := lc.filter(ps.str("ValidTgts"))
	if err != nil {
		return false, err
	}
	if len(pf.Clauses) != 0 || pf.Players == 0 {
		return false, refuse("library pick ValidTgts$ %q", ps.str("ValidTgts"))
	}
	lc.addTarget(lTarget{F: pf})
	ps.consume("TgtPrompt")
	// The re-spelled Chooser$ Opponent answers as the controller's first
	// living opponent, gorge's as the announced target: the two are the same
	// seat only at two players, so this caller (not libSearchOwner, which
	// reads the genuine spelling) marks the card two-seat only.
	lc.twoSeat("Chooser$ Targeted on a library pick answered by the targeted opponent")
	ps.m["Chooser"] = "Opponent"
	return true, nil
}

// libSweepShuffle lowers ChangeZoneAll Shuffle$ True out of the library into
// another zone over the cards your own earlier library search remembered
// (Intuition's "put the rest into your graveyard. Then shuffle"). gorge's
// changeZoneAllShuffleTail shuffles the owner of each library a moved card
// left, so only a sweep that moved something shuffles, and every remembered
// card is yours: the shuffle is yours, after the sweep, behind a count of the
// remembered cards still in the library. Any other library sweep refuses.
func (lc *lowerCtx) libSweepShuffle(in lInstr, dest int) error {
	if in.Obj.Kind != rNone || in.F == nil || len(in.F.Clauses) == 0 || in.F.Players != 0 || in.Note != "" ||
		lc.czForgetSet || !lc.rememberedYourSearch() {
		return refuse("ChangeZoneAll Shuffle$ from the library to %s", zoneLabel[dest])
	}
	for _, c := range in.F.Clauses {
		if c.Flags&fRemembered == 0 {
			return refuse("ChangeZoneAll Shuffle$ from the library to %s beyond the remembered cards", zoneLabel[dest])
		}
	}
	count := lCond{Kind: cValue, A: lValue{Kind: vCountHand, N: 1, F: in.F}, Op: opGE, B: cst(1)}
	lc.emit(lInstr{Op: "SkipIf", Cond: count, Skip: 2, Note: "gorge shuffles only the libraries a swept card left"})
	lc.emit(in)
	lc.emit(lInstr{Op: "Shuffle", Obj: lRef{Kind: rYou}})
	return nil
}

// rememberedYourSearch reports whether the program so far remembers the
// cards of a search of your own library (a Search obj=You with Remember).
func (lc *lowerCtx) rememberedYourSearch() bool {
	for _, p := range lc.ab.Prog {
		if p.Op == "Search" && p.Obj.Kind == rYou && strings.Contains(p.CS, "Remember") {
			return true
		}
	}
	return false
}
