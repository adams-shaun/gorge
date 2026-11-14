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
	if ps.str("Choices") == "Player.Opponent+!EnchantedBy" {
		return lc.choosePlayerRandom(ps) // the Maddening Hex shape
	}
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
	lc.twoSeat("ChoosePlayer among the opponents answered as the one opponent")
	if v := ps.str("Defined"); v != "" && v != "You" {
		return refuse("ChoosePlayer Defined$ %q", v)
	}
	// prompt and AI only
	ps.consume("ChoiceTitle", "AILogic")
	lc.chosenOppAb = lc.ab
	return nil
}

// choosePlayerRandom lowers ChoosePlayer Choices$ "Player.Opponent+!EnchantedBy"
// with Random$ True (Maddening Hex's re-attach): gorge effChoosePlayer builds
// the pool over each chooser (Defined$ You, the resolving controller) —
// MatchesPlayerSpecWithSVars: the living opponents of the chooser that the
// source Aura is not attached to (!EnchantedBy is player.isEnchantedBy(source);
// a source off the battlefield fails closed and excludes nobody) — and
// Random$ True records ONE pick with the engine rng (gorge randomChoices: one
// rng.IntN(len(pool)) draw over the AliveFrom order), never a decision; an
// empty pool draws nothing and records no choice. The bounds are choiceBounds'
// default (min = max = Amount$ 1). The pick is the resolution's chosen player
// (kop OpChoosePlayer, RefResChosenPlayer), which this ability's ChosenPlayer
// reads take — Maddening Hex's Attach to the chosen player.
func (lc *lowerCtx) choosePlayerRandom(ps *params) error {
	if v := ps.str("Defined"); v != "" && v != "You" {
		return refuse("ChoosePlayer Defined$ %q", v)
	}
	if !ps.is("Random", "True") {
		return refuse("ChoosePlayer Choices$ %q without Random$ True", ps.str("Choices"))
	}
	if v := ps.str("Amount"); v != "" && v != "1" {
		return refuse("ChoosePlayer Amount$ %q", v)
	}
	if v := ps.str("MinAmount"); v != "" && v != "1" {
		return refuse("ChoosePlayer MinAmount$ %q", v)
	}
	// prompt and AI only
	ps.consume("ChoiceTitle", "AILogic", "Random", "Defined", "Choices")
	lc.emit(lInstr{Op: "ChoosePlayer", Obj: lRef{Kind: rYou}, CS: "Random"})
	lc.chosenPlAb = lc.ab
	return nil
}

// chosenPlayerDefined maps ChosenPlayer / Player.Chosen to the opponent
// while this ability's one-opponent ChoosePlayer is armed.
func (lc *lowerCtx) chosenPlayerDefined(v string) (lRef, bool) {
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

// clearChosenPlayer consumes Cleanup ClearChosenPlayer$ True while a choice
// is armed in this ability — the one-opponent choice or the random-pool one:
// it clears gorge's choice (Ctx.Chosen's player entries and the source's
// event-backed list, whose only readers here are gone) and changes no state
// the kernel models. Elsewhere the param stays unread and refuses.
func (lc *lowerCtx) clearChosenPlayer(ps *params) {
	armed := (lc.chosenOppAb != nil && lc.chosenOppAb == lc.ab) ||
		(lc.chosenPlAb != nil && lc.chosenPlAb == lc.ab)
	if armed && ps.m["ClearChosenPlayer"] == "True" {
		ps.consume("ClearChosenPlayer")
		lc.chosenOppAb = nil
		lc.chosenPlAb = nil
	}
}
