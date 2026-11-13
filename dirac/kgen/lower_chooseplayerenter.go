package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The as-enters chosen opponent (The Rack, Black Vise): an ETB replacement
// whose whole body is `ChoosePlayer | Defined$ You | Choices$
// Player.Opponent`. gorge runs it through effChoosePlayer as the permanent
// enters (a KChoose of one "player" option per opponent, AliveFrom order) and
// records the answer on the permanent's Object.Chosen, cleared when it leaves
// the battlefield (CR 400.7). It lowers to OpExt ExtChoosePlayer, the enters
// program's only instruction; the permanent's own battlefield triggers read
// the choice, and so do its printed Continuous statics (Entropic Specter,
// Nyxathid):
//
//   - ValidPlayer$ Player.Chosen on a Phase trigger: kop.WhoseChosen
//   - Defined$ ChosenPlayer: kop.RefChosenPlayer
//   - Count$ValidHand Card.ChosenCtrl: the chosen player's hand
//     (a hand card's controller is its owner), ExHandSizeOf over
//     RefChosenPlayer; no chosen player counts none, as gorge's
//     chosenCtrlMatches fails closed
//
// Anywhere else the choice is unread and the spellings keep refusing.

// whoChosen is the trigger Whose of ValidPlayer$ Player.Chosen. build_ab.go
// casts the value to kop.Whose, so it must equal kop.WhoseChosen
// (TestChoosePlayerEnterLowers).
const whoChosen uint8 = 201

var _ = [1]struct{}{}[int(whoChosen)-int(kop.WhoseChosen)]

// entersChoosingPlayer reports whether face f chooses an opponent as it
// enters (the one-node ETB ChoosePlayer body etbChoosePlayer lowers).
func entersChoosingPlayer(f *cards.Face) bool {
	if f == nil {
		return false
	}
	for i := range f.Repls {
		r := &f.Repls[i]
		if r.With != nil && r.ParamStr(cards.PKKeyword) == "ETBReplacement" && isEnterChoosePlayer(r.With) {
			return true
		}
	}
	return false
}

// isEnterChoosePlayer is the exact as-enters opponent choice body.
func isEnterChoosePlayer(sa *cards.SA) bool {
	if sa.API != "ChoosePlayer" || sa.Sub != nil {
		return false
	}
	switch strings.TrimSpace(sa.Params["Choices"]) {
	case "Player.Opponent", "Opponent":
	default:
		return false
	}
	if d, ok := sa.Params["Defined"]; ok && strings.TrimSpace(d) != "You" {
		return false
	}
	for k := range sa.Params {
		switch k {
		case "Choices", "Defined", "ChoiceTitle", "AILogic", "SpellDescription":
		default:
			return false
		}
	}
	return true
}

// etbChoosePlayer lowers the as-enters opponent choice: the entering
// permanent's own, unconditional, sole ETB replacement part.
func (lc *lowerCtx) etbChoosePlayer(ps *params, node, sa *cards.SA, other bool, cond *lCond) error {
	if other || node != sa || cond.Kind != cNone || len(lc.ab.Prog) != 0 || !isEnterChoosePlayer(sa) {
		return refuse("ETB replacement api ChoosePlayer")
	}
	ps.consume("Choices", "Defined", "ChoiceTitle", "AILogic", "SpellDescription") // AI and prompt only
	lc.emit(lInstr{Op: "ChoosePlayerEnter", Obj: lRef{Kind: rYou}, Note: "choice of an opponent as it enters"})
	return nil
}

// enterChosenRead reports whether the ability being lowered is the face's own
// printed battlefield trigger or printed Continuous static on a card that
// chooses an opponent as it enters: its source is that permanent, so the
// choice reads its chosen byte (a static off the battlefield, a
// characteristic-defining one in a hand, has no permanent and reads none, as
// gorge's choice-less object does).
func (lc *lowerCtx) enterChosenRead() bool {
	if lc.granted != 0 || !entersChoosingPlayer(lc.face) {
		return false
	}
	if lc.exwStatic {
		return true // lowerStatic: a printed Mode$ Continuous static
	}
	if lc.ab == nil || lc.ab.Kind != akTriggered || lc.ab.Latent {
		return false
	}
	t := lc.ab.Trigger
	return t != nil && t.Zone == zBattlefield
}

// chosenPlayerEnterDefined maps Defined$ ChosenPlayer / Player.Chosen to
// RefChosenPlayer where enterChosenRead holds.
func (lc *lowerCtx) chosenPlayerEnterDefined(v string) (lRef, bool) {
	if (v == "ChosenPlayer" || v == "Player.Chosen") && lc.enterChosenRead() {
		return lRef{Kind: rChosenPlayer}, true
	}
	return lRef{}, false
}

// chosenCtrlHand lowers Count$ValidHand Card.ChosenCtrl where
// enterChosenRead holds: the cards in the chosen player's hand (N 1: cards,
// as gorge's zone count reads).
func (lc *lowerCtx) chosenCtrlHand(rest string) (lValue, bool) {
	if rest != "ValidHand Card.ChosenCtrl" || !lc.enterChosenRead() {
		return lValue{}, false
	}
	return lValue{Kind: vHandSizeOf, N: 1, Ref: lRef{Kind: rChosenPlayer}}, true
}
