package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// The api: effects for modes, branches, repeats, delayed triggers, effects and turn structure (lower_api.go).
func init() {
	apiOp("Vote", (*lowerCtx).vote) // lower_vote.go
	apiPS("Charm", (*lowerCtx).charm)
	apiOp("Effect", (*lowerCtx).apiEffect)
	apiPS("DelayedTrigger", (*lowerCtx).delayedTrigger)
	apiPS("ImmediateTrigger", (*lowerCtx).immediateTrigger)
	apiOp("Cleanup", (*lowerCtx).apiCleanup)
	apiPS("GenericChoice", (*lowerCtx).genericChoice)
	apiOp("AddTurn", (*lowerCtx).apiAddTurn)
	apiOp("EndTurn", (*lowerCtx).apiEndTurn)
	apiPS("Repeat", (*lowerCtx).repeat)
	apiPS("StoreSVar", (*lowerCtx).storeSVar)
	apiPS("Demonstrate", (*lowerCtx).demonstrate)
	apiPS("FlipCoin", (*lowerCtx).flipCoin)
	apiPS("Clash", (*lowerCtx).clash) // lower_clash.go
	apiPS("Branch", (*lowerCtx).branch)
	apiPS("RepeatEach", (*lowerCtx).repeatEachPlayers)
	apiPS("RollDice", (*lowerCtx).rollDice)
	apiPS("AddPhase", (*lowerCtx).addPhase)           // lower_addphase.go
	apiPS("SkipPhase", (*lowerCtx).skipPhase)         // lower_skipphase.go
	apiPS("ChangeTargets", (*lowerCtx).changeTargets) // lower_stackrewrite.go
}

func (lc *lowerCtx) apiEffect(sa *cards.SA, ps *params) error {
	if lc.prevColor {
		return lc.preventColorEffect(ps)
	}
	if lc.chosenSrc {
		return lc.chosenSrcEffect(ps) // lower_choosesource.go
	}
	if lc.colGrant {
		return lc.colorGrantEffect(ps) // (Skrelv)
	}
	if lc.clAt > 0 {
		return lc.counterLoseEffect(ps) // (Tishana's Tidebinder)
	}
	return lc.effectAPI(ps)
}

func (lc *lowerCtx) apiCleanup(sa *cards.SA, ps *params) error {
	// A face that keeps its Imprinted on the source clears it first: gorge's
	// walker clears after the SA's body and its other riders, and the
	// kernel's ClearImprinted runs before the rest of this Cleanup.
	impCleared := false
	if lc.impPersist && ps.is("ClearImprinted", "True") {
		ps.consume("ClearImprinted")
		lc.emit(lInstr{Op: "ClearImprinted"})
		impCleared = true
	}
	if ps.is("ClearRemembered", "True") {
		lc.emit(lInstr{Op: "ClearRemembered"})
	}
	if ps.is("ClearChosenCard", "True") {
		lc.emit(lInstr{Op: "ClearChosen"})
	}
	if err := lc.clearNamedCard(ps); err != nil { // lower_namecard.go
		return err
	}
	lc.clearChosenPlayer(ps) // lower_definedheads.go
	if v, ok := ps.get("ClearChosenColor"); ok {
		// the chosen-colour grants read the colour at resolution (gorge
		// definedKWGrant, freezeChosenColor): clearing it after them
		// changes nothing
		if v != "True" || !lc.colGrant {
			return refuse("Cleanup ClearChosenColor$ outside a chosen-colour grant")
		}
		lc.colGrant = false
	}
	if !impCleared && lc.breachStage == 2 && ps.is("ClearImprinted", "True") && ps.is("ClearChosenCard", "True") {
		ps.consume("ClearImprinted") // Breach the Multiverse: the chosen set is the imprint (lower_repeatgravechoice.go)
		lc.breachStage = 0
	}
	if !impCleared && lc.exileLandStage == 2 && ps.is("ClearImprinted", "True") && ps.is("ClearChosenCard", "True") && ps.is("ClearRemembered", "True") {
		ps.consume("ClearImprinted") // Ensign Pavel Chekov: the chosen set is the imprint (lower_repeatexilechoice.go)
		lc.exileLandStage = 0
	}
	if !impCleared && (lc.digImprintSetLive() || lc.repeatImprintBody) && ps.is("ClearImprinted", "True") {
		ps.consume("ClearImprinted") // the Dig's imprinted set (lower_digimprintset.go); or a RepeatDefined$ Imprinted body's per-pass clear before its Dig
		lc.emit(lInstr{Op: "ClearImprinted"})
	}
	if !impCleared && lc.digImprinted && ps.is("ClearImprinted", "True") {
		// the imprinted card is the kernel's remembered set
		// (lower_digimprint.go): ClearRemembered$ beside it clears it
		if !ps.is("ClearRemembered", "True") {
			return refuse("Cleanup ClearImprinted$ without ClearRemembered$")
		}
	}
	if !impCleared && lc.digImprintChosen && ps.is("ClearImprinted", "True") {
		// the imprint is the kernel's chosen set (lower_digimprint.go):
		// gorge's seek-found Imprint "clear" clears it
		ps.consume("ClearImprinted")
		lc.emit(lInstr{Op: "ClearChosen"})
	}
	return nil
}

func (lc *lowerCtx) apiAddTurn(sa *cards.SA, ps *params) error {
	n, err := lc.amount(ps, "NumTurns", 1)
	if err != nil {
		return err
	}
	if !n.isConst() || n.N != 1 {
		return refuse("AddTurn %v", n)
	}
	if d, ok := ps.get("Defined"); ok && strings.TrimSpace(d) != "You" {
		return refuse("AddTurn Defined$ %q", d)
	}
	lc.emit(lInstr{Op: "AddTurn", Obj: lRef{Kind: rYou}})
	return nil
}

func (lc *lowerCtx) apiEndTurn(sa *cards.SA, ps *params) error {
	lc.emit(lInstr{Op: "EndTurn"})
	return nil
}
