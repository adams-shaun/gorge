package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

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
	if ps.str("Choices") == "Player.Opponent+!EnchantedBy" {
		return lc.choosePlayerRandom(ps, "Random") // the Maddening Hex shape
	}
	high := 0
	switch ps.str("Choices") {
	case "Player.Opponent", "Opponent", "Player.Other":
		if ps.is("Random", "True") && ps.str("Choices") != "Player.Other" {
			// a random pick among every living opponent (Knight Rampager,
			// Raving Dead, The Ruinous Powers, ...): kop ChPlPlain, no Aura
			// exclusion
			return lc.choosePlayerRandom(ps, "RandomOpp")
		}
	case "Player.Opponent+lifeEQX":
		// the opponents tied at the highest life total (Your Inescapable Doom):
		// gorge reads the threshold X over the living opponents of the
		// controller, the pool is theirs at that life, from the chooser
		if !mid {
			return refuse("ChoosePlayer Choices$ %q whose chain never reads the choice", ps.str("Choices"))
		}
		switch x := strings.TrimSpace(lc.face.SVars["X"]); x {
		case "PlayerCountOpponents$HighestLifeTotal", "Count$OppGreatestLifeTotal":
			high = kop.ChoosePlayerMidHigh
		default:
			return refuse("ChoosePlayer Choices$ %q with X %q", ps.str("Choices"), x)
		}
	case "Player", "Player.!IsRemembered", "":
		return lc.choosePlayerRes(ps) // lower_chooseplayerres.go
	case "Player.OpponentOf Remembered":
		// the repeated player's opponent choice (Bend or Break): only inside
		// a RepeatEach player body whose chooser is the repeated player
		if lc.repeatPl > 0 && ps.str("Defined") == "Remembered" {
			return lc.choosePlayerEach(ps)
		}
		return refuse("ChoosePlayer Choices$ %q", ps.str("Choices"))
	default:
		return refuse("ChoosePlayer Choices$ %q", ps.str("Choices"))
	}
	if ps.is("Random", "True") {
		// the asked pools below pose a decision; a random pick over any
		// other spelling is a different shape (Player.Other)
		return refuse("ChoosePlayer Random$ True over Choices$ %q", ps.str("Choices"))
	}
	if err := lc.choosePlayerShape(ps); err != nil {
		return err
	}
	// the mid-resolution ask is among the OPPONENTS (kop.oppsFrom); a
	// Player.Other pool of the resolving controller is the same players in
	// the same AliveFrom order (gorge models no teams: every other player is
	// an opponent), so it lowers the same (Discerning Financier)
	if mid {
		// the chain reads the choice: gorge asks it mid-resolution whatever
		// the seat count, so the kernel does too (OpExt ExtChoosePlayerMid,
		// a sole opponent recorded unasked) and the card holds at any pod
		lc.emit(lInstr{Op: "ChoosePlayerMid", C: high, Note: "a mid-resolution choice of an opponent, read by this chain"})
		lc.chosenMidAb = lc.ab
		return nil
	}
	lc.twoSeat("ChoosePlayer among the opponents answered as the one opponent")
	lc.chosenOppAb = lc.ab
	return nil
}

// choosePlayerEach lowers a RepeatEach player body's ChoosePlayer whose
// chooser is the repeated player and pool their opponents (Bend or Break:
// Defined$ Remembered, Choices$ Player.OpponentOf Remembered; gorge
// effChoosePlayer pools over the remembered player's AliveFrom matching
// Player.OpponentOf Remembered): that player chooses one of their opponents,
// kept in the resolution (kop ExtChoosePlayerEach, res.chPl) for the chain's
// TwoPiles Chooser$ ChosenPlayer (lower_twopiles.go twoPilesEach). With a
// single opponent it is recorded unasked (the oracle's forcedPlayerChoice);
// with none gorge records an empty choice, which the readers find nothing in.
// RememberChosen$ and any other rider refuse.
func (lc *lowerCtx) choosePlayerEach(ps *params) error {
	if v := ps.str("Defined"); v != "Remembered" {
		return refuse("ChoosePlayer Defined$ %q", v)
	}
	// the chain's rider, prompt and AI metadata only
	ps.consume("Choices", "Defined", "SubAbility", "ChoiceTitle", "AILogic")
	if left := ps.unread(); len(left) > 0 {
		return refuse("ChoosePlayer unread params %v", left)
	}
	lc.chosenEachAb = lc.ab
	lc.emit(lInstr{Op: "ChoosePlayerEach", Note: "the repeated player chooses one of their opponents for the chain's TwoPiles"})
	return nil
}

// choosePlayerOpponents validates and consumes the prompt-side shape of a
// ChoosePlayer whose pool is the one opponent (choosePlayer's and
// choosePlayerPod's shared head): Choices$ an opponent spelling, no
// mid-resolution choice beside it, Defined$ You, prompt and AI only. The
// Player / Player.!IsRemembered spellings stay with choosePlayer, which hands
// them to choosePlayerRes and stops there.
func (lc *lowerCtx) choosePlayerOpponents(ps *params) error {
	switch ps.str("Choices") {
	case "Player.Opponent", "Opponent", "Player.Other":
	default:
		return refuse("ChoosePlayer Choices$ %q", ps.str("Choices"))
	}
	return lc.choosePlayerShape(ps)
}

// choosePlayerShape is the rest of choosePlayerOpponents, past the Choices$
// spelling: no mid-resolution choice beside it, Defined$ You, prompt and AI
// only. choosePlayer's highest-life pool (Player.Opponent+lifeEQX) enters here
// directly, since that spelling is its own.
func (lc *lowerCtx) choosePlayerShape(ps *params) error {
	if lc.chosenResAb != nil && lc.chosenResAb == lc.ab {
		return refuse("the one-opponent ChoosePlayer beside a mid-resolution ChoosePlayer")
	}
	if v := ps.str("Defined"); v != "" && v != "You" {
		return refuse("ChoosePlayer Defined$ %q", v)
	}
	// prompt and AI only
	ps.consume("ChoiceTitle", "AILogic")
	return nil
}

// choosePlayerPod lowers the as-enters opponent choice that opens a chain
// whose next links read it (Sorcerous Spyglass: ChoosePlayer -> RevealHand
// Look$ over the chosen player -> NameCard -> Cleanup ClearChosenPlayer$):
// gorge runs the whole chain in the entry's Updated body, so it asks the
// choice among the opponents with the permanent on the battlefield (OpExt
// ExtChoosePlayer, a sole opponent recorded unasked) at any seat count, and
// the card holds in a pod. The look-then-name ask follows it in the same
// enters program (lower_choosename.go, ExtChooseName B = 2) and reads the
// choice while it lasts: the name overwrites the chosen byte the player
// rode, as gorge's Cleanup clears the choice at the same point.
func (lc *lowerCtx) choosePlayerPod(ps *params) error {
	if err := lc.choosePlayerOpponents(ps); err != nil {
		return err
	}
	lc.emit(lInstr{Op: "ChoosePlayerEnter", Obj: lRef{Kind: rYou}, Note: "choice of an opponent as it enters, ahead of the look-then-name ask"})
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
//
// cs is the instruction's build form: "Random" is Maddening Hex's pool (the
// bearer left out), "RandomOpp" the plain living opponents (Choices$
// Player.Opponent / Opponent, kop.ChPlPlain: the same rng draw over the
// AliveFrom order).
func (lc *lowerCtx) choosePlayerRandom(ps *params, cs string) error {
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
	lc.emit(lInstr{Op: "ChoosePlayer", Obj: lRef{Kind: rYou}, CS: cs})
	lc.chosenPlAb = lc.ab
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
	if lc.chosenPlAb != nil && lc.chosenPlAb == lc.ab && (v == "ChosenPlayer" || v == "Player.Chosen") {
		return lRef{Kind: rResChosenPlayer}, true // the random pick (choosePlayerRandom)
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
// the kernel models. While the mid-resolution choice is armed it is the same
// clear, and readers past the Cleanup must find none (the resolution's answer
// dies with the resolution). Elsewhere the param stays unread and refuses.
func (lc *lowerCtx) clearChosenPlayer(ps *params) {
	armed := (lc.chosenOppAb != nil && lc.chosenOppAb == lc.ab) ||
		(lc.chosenPlAb != nil && lc.chosenPlAb == lc.ab) ||
		(lc.chosenEachAb != nil && lc.chosenEachAb == lc.ab)
	if armed && ps.m["ClearChosenPlayer"] == "True" {
		ps.consume("ClearChosenPlayer")
		lc.chosenOppAb = nil
		lc.chosenPlAb = nil
		lc.chosenEachAb = nil
	}
	if lc.chosenMidAb != nil && lc.chosenMidAb == lc.ab && ps.m["ClearChosenPlayer"] == "True" {
		ps.consume("ClearChosenPlayer")
		lc.chosenMidAb = nil
	}
}
