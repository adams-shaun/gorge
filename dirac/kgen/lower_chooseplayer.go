package kgen

// the cold lowering helpers for ChoosePlayer's one
// opponent, and the qualified and joined TriggeredCard Defined$ spellings.

// choosePlayer lowers a ChoosePlayer whose pool is exactly the one opponent
// (gorge effChoosePlayer, choose_control.go: the living players matching
// Choices$ from the chooser, a KChoose of Min = Max = 1). A chain whose
// later links read the choice lowers to OpExt ExtChoosePlayerMid: gorge
// asks the choice mid-resolution at any seat count, so the kernel does too,
// and the readers map to RefChosenPlayer (lower_chooseplayermid.go). In two
// players Choices$ Player.Opponent / Opponent / Player.Other leave one
// option, which the lockstep answers in place (PassElision), so the kernel
// poses nothing: the chosen player IS the opponent. That path emits no
// instruction and arms ChosenPlayer for the rest of this ability; any other
// shape refuses.
func (lc *lowerCtx) choosePlayer(ps *params, mid bool) error {
	switch ps.str("Choices") {
	case "Player.Opponent", "Opponent", "Player.Other":
	case "Player", "Player.!IsRemembered":
		return lc.choosePlayerRes(ps) // lower_chooseplayerres.go
	default:
		return refuse("ChoosePlayer Choices$ %q", ps.str("Choices"))
	}
	if lc.chosenResAb != nil && lc.chosenResAb == lc.ab {
		return refuse("the one-opponent ChoosePlayer beside a mid-resolution ChoosePlayer")
	}
	if v := ps.str("Defined"); v != "" && v != "You" {
		return refuse("ChoosePlayer Defined$ %q", v)
	}
	// prompt and AI only
	ps.consume("ChoiceTitle", "AILogic")
	// the mid-resolution ask is among the OPPONENTS (kop.oppsFrom); a
	// Player.Other pool is any other player, still two-seat-only
	if ps.str("Choices") != "Player.Other" && mid {
		// the chain reads the choice: gorge asks it mid-resolution whatever
		// the seat count, so the kernel does too (OpExt ExtChoosePlayerMid,
		// a sole opponent recorded unasked) and the card holds at any pod
		lc.emit(lInstr{Op: "ChoosePlayerMid", Note: "a mid-resolution choice of an opponent, read by this chain"})
		lc.chosenMidAb = lc.ab
		return nil
	}
	lc.twoSeat("ChoosePlayer among the opponents answered as the one opponent")
	lc.chosenOppAb = lc.ab
	return nil
}

// chosenPlayerDefined maps ChosenPlayer / Player.Chosen to the opponent
// while this ability's one-opponent ChoosePlayer is armed.
func (lc *lowerCtx) chosenPlayerDefined(v string) (lRef, bool) {
	if r, ok := lc.chosenPlayerMidDefined(v); ok {
		return r, true // lower_chooseplayermid.go: the resolution's choice
	}
	if r, ok := lc.chosenPlayerResDefined(v); ok {
		return r, true // lower_chooseplayerres.go
	}
	if lc.chosenOppAb == nil || lc.chosenOppAb != lc.ab {
		return lRef{}, false
	}
	if v == "ChosenPlayer" || v == "Player.Chosen" {
		return lRef{Kind: rEachOpp}, true
	}
	return lRef{}, false
}

// clearChosenPlayer consumes Cleanup ClearChosenPlayer$ True while the
// one-opponent choice is armed: it clears gorge's choice and changes no
// state the kernel models. While the mid-resolution choice is armed it is
// the same clear, and readers past the Cleanup must find none (the
// resolution's answer dies with the resolution). Elsewhere the param stays
// unread and refuses.
func (lc *lowerCtx) clearChosenPlayer(ps *params) {
	if lc.chosenOppAb != nil && lc.chosenOppAb == lc.ab && ps.m["ClearChosenPlayer"] == "True" {
		ps.consume("ClearChosenPlayer")
		lc.chosenOppAb = nil
	}
	if lc.chosenMidAb != nil && lc.chosenMidAb == lc.ab && ps.m["ClearChosenPlayer"] == "True" {
		ps.consume("ClearChosenPlayer")
		lc.chosenMidAb = nil
	}
}
