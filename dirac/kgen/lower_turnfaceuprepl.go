package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// turnUpRepl lowers an R:Event$ TurnFaceUp line.
//
// gorge replacementMatches (engine/rules/replacement_match.go
// cards.ReplTurnFaceUp) matches the permanent being turned up in its
// pre-flip state, reads ValidCard$ in the replacement source's frame, and
// gates on PlayerTurn$ (the source's controller is the active player).
//   - With a ReplaceWith$ body the body runs before the turn-up event folds
//     (applyTurnFaceUpReplacements, replacement_phase.go): the counters land
//     while the permanent is still face down. Only Card.Self with exactly one
//     PutCounter on Self lowers (Bubble Smuggler, Crowd-Control Warden).
//   - Layer$ CantHappen without a body makes the special action illegal
//     (turnFaceUpCantHappen): Karlov Watchdog, Unable to Scream.
func (lc *lowerCtx) turnUpRepl(rp *params, with *cards.SA, rl *lRepl) error {
	const shape = "TurnFaceUp replacement shape"
	if rp.str("ActiveZones") != "Battlefield" {
		return refuse(shape)
	}
	vc := rp.str("ValidCard")
	if with != nil {
		// body shape: no Layer$, no Optional$, one PutCounter on Self
		if _, ok := rp.m["Layer"]; ok || vc != "Card.Self" || with.API != "PutCounter" || with.Sub != nil {
			return refuse(shape)
		}
		rp.consume("Secondary") // display-only (Crowd-Control Warden's second line)
		ps := newParams(with.Params)
		if ps.str("Defined") != "Self" || ps.str("CounterType") == "" {
			return refuse(shape)
		}
		for _, k := range []string{"CounterTypes", "ETB", "UpTo", "Optional", "Bolster", "Support", "Adapt", "Monstrosity", "DividedAsYouChoose"} {
			if _, ok := ps.m[k]; ok {
				return refuse(shape)
			}
		}
		n, err := lc.amount(ps, "CounterNum", 1)
		if err != nil {
			return err
		}
		ct := ps.str("CounterType")
		if ct == "ENERGY" || ct == "EachFromSource" {
			return refuse(shape)
		}
		ab := &lAbility{Kind: akLatent}
		if err := lc.withAbility(ab, func() error {
			lc.emit(lInstr{Op: "PutCounter", Obj: lRef{Kind: rReplaced}, A: n, CS: ct})
			return nil
		}); err != nil {
			return err
		}
		if left := ps.unread(); len(left) > 0 {
			return refuse("TurnFaceUp counter body unread params %v", left)
		}
		rl.Prog = ab.Prog
		rl.Kind = "TurnUpProgram"
		return nil
	}
	if rp.str("Layer") != "CantHappen" {
		return refuse(shape)
	}
	// gorge matches the event object pre-flip and consults the predicate only
	// for face-down candidates (legal_walk_turnup.go), so a trailing +faceDown
	// is always true at match time
	vc = strings.TrimSuffix(vc, "+faceDown")
	f, err := lc.filter(vc)
	if err != nil {
		return err
	}
	rl.F = f
	if v, ok := rp.get("PlayerTurn"); ok {
		if v != "True" {
			return refuse(shape)
		}
		rl.Cond = lCond{Kind: cYourTurn}
	}
	rl.Kind = "CantTurnUp"
	return nil
}
