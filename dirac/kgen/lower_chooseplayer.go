package kgen

// the cold lowering helpers for ChoosePlayer's one
// opponent, and the qualified and joined TriggeredCard Defined$ spellings.

// choosePlayer lowers a ChoosePlayer whose pool is exactly the one opponent
// (gorge effChoosePlayer, choose_control.go: the living players matching
// Choices$ from the chooser, a KChoose of Min = Max = 1). In two players
// Choices$ Player.Opponent / Opponent / Player.Other leave one option, which
// the lockstep answers in place (PassElision), so the kernel poses nothing:
// the chosen player IS the opponent. It emits no instruction and arms
// ChosenPlayer for the rest of this ability; any other shape refuses.
func (lc *lowerCtx) choosePlayer(ps *params) error {
	switch ps.str("Choices") {
	case "Player.Opponent", "Opponent", "Player.Other":
	default:
		return refuse("ChoosePlayer Choices$ %q", ps.str("Choices"))
	}
	lc.twoSeat("ChoosePlayer among the opponents answered as the one opponent")
	if v := ps.str("Defined"); v != "" && v != "You" {
		return refuse("ChoosePlayer Defined$ %q", v)
	}
	// prompt and AI only
	ps.consume("ChoiceTitle", "AILogic")
	lc.chosenOppAb = lc.ab
	return nil
}

// chosenPlayerDefined maps ChosenPlayer / Player.Chosen to the opponent
// while this ability's one-opponent ChoosePlayer is armed.
func (lc *lowerCtx) chosenPlayerDefined(v string) (lRef, bool) {
	if lc.chosenOppAb == nil || lc.chosenOppAb != lc.ab {
		return lc.srcChosenDefined(v) // lower_chosenopp.go
	}
	if v == "ChosenPlayer" || v == "Player.Chosen" {
		return lRef{Kind: rEachOpp}, true
	}
	return lRef{}, false
}

// clearChosenPlayer consumes Cleanup ClearChosenPlayer$ True while the
// one-opponent choice is armed: it clears gorge's choice and changes no
// state the kernel models. Elsewhere the param stays unread and refuses.
func (lc *lowerCtx) clearChosenPlayer(ps *params) {
	if lc.chosenOppAb != nil && lc.chosenOppAb == lc.ab && ps.m["ClearChosenPlayer"] == "True" {
		ps.consume("ClearChosenPlayer")
		lc.chosenOppAb = nil
	}
}
