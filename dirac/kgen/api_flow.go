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
	if done, err := lc.monarchJailEffect(ps); done || err != nil {
		return err // lower_monarchjail.go
	}
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
	if !impCleared && !lc.digImprintSetLive() && lc.repeatGateImprinted() && ps.is("ClearImprinted", "True") {
		// Eureka's per-pass clear, before the pass's imprint writer: the
		// Repeat's check gate counts the set (lower_imprintzone.go)
		if err := lc.openImprint("Cleanup ClearImprinted$"); err != nil {
			return err
		}
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
	in := lInstr{Op: "AddTurn", Obj: lRef{Kind: rYou}}
	if err := lc.addTurnRider(ps, &in); err != nil {
		return err
	}
	lc.emit(in)
	return nil
}

// addTurnRider lowers ExtraTurnDelayedTrigger$ with
// ExtraTurnDelayedTriggerExecute$ (Final Fortune, Last Chance, Warrior's
// Oath, Chance for Glory's sub-ability): the granted turn's end step fires
// the Execute body. Only the DelTrig phase trigger whose Execute is DB$
// LosesGame on You lowers; gorge registers the delayed trigger at the -1
// ExtraTurn consumption with the granted turn as MinTurn (engine/events/
// apply_turn.go foldExtraTurn), and the kernel arms the rider when the
// granted turn begins (act_extraturn.go exRideArm), so the granting turn's own
// end step never fires it. Every other shape refuses: Alchemist's Gambit's
// and Kang the Conqueror's Execute is DB$ Effect, whose static modes are
// unmodelled.
func (lc *lowerCtx) addTurnRider(ps *params, in *lInstr) error {
	name, ok := ps.get("ExtraTurnDelayedTrigger")
	if !ok {
		if _, ok := ps.get("ExtraTurnDelayedTriggerExecute"); ok {
			return refuse("AddTurn ExtraTurnDelayedTriggerExecute$ without ExtraTurnDelayedTrigger$")
		}
		return nil
	}
	exec, ok := ps.get("ExtraTurnDelayedTriggerExecute")
	if !ok {
		return refuse("AddTurn ExtraTurnDelayedTrigger$ %q without ExtraTurnDelayedTriggerExecute$", name)
	}
	dt := cards.ResolveSVar(lc.face.SVars, name)
	var dp *params
	if dt != nil {
		dp = newParams(dt.Params)
	} else if raw, ok := lc.face.SVars[name]; ok {
		// the raw T: shape (Final Fortune's "ThisTurn$ True | Mode$ Phase |
		// ..."): gorge's delayedTriggerSpec parses the named body through
		// ParseTriggerLine when ResolveSVar has no DB$ head for it
		if trig, ok := cards.ParseTriggerLine(raw); ok {
			dp = newParams(trig.Params)
		}
	}
	if dp == nil {
		return refuse("AddTurn ExtraTurnDelayedTrigger$ %q: no such SVar", name)
	}
	if dp.str("Mode") != "Phase" {
		return refuse("AddTurn rider %s: Mode$ %q", name, dp.str("Mode"))
	}
	if dp.str("Phase") != "End of Turn" {
		return refuse("AddTurn rider %s: Phase$ %q", name, dp.str("Phase"))
	}
	if t := dp.str("ThisTurn"); t != "True" {
		return refuse("AddTurn rider %s: ThisTurn$ %q", name, t)
	}
	if _, ok := dp.get("Static"); ok {
		return refuse("AddTurn rider %s: Static$ rider", name)
	}
	ex := cards.ResolveSVar(lc.face.SVars, exec)
	if ex == nil {
		return refuse("AddTurn rider Execute$ %q: no such SVar", exec)
	}
	if ex.API != "LosesGame" {
		return refuse("AddTurn rider Execute$ %s: api %s", exec, ex.API)
	}
	ep := newParams(ex.Params)
	if d := strings.TrimSpace(ep.str("Defined")); d != "" && d != "You" {
		return refuse("AddTurn rider Execute$ %s: Defined$ %q", exec, d)
	}
	in.CS = "ExtraTurnRider" // opAddTurn: C names the granted turn's end step (kop.StepEnd)
	return nil
}

func (lc *lowerCtx) apiEndTurn(sa *cards.SA, ps *params) error {
	lc.emit(lInstr{Op: "EndTurn"})
	return nil
}
