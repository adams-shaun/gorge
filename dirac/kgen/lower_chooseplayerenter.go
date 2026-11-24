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

// whoEnchanted is the trigger Whose of Player.EnchantedBy (a Phase line's
// ValidPlayer$, a DamageDone line's ValidTarget$): the player the source
// Aura enchants. It must equal kop.WhoseEnchanted (build_ab.go casts it).
const whoEnchanted uint8 = 202

var _ = [1]struct{}{}[int(whoEnchanted)-int(kop.WhoseEnchanted)]

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

// replChainChoosePlayer is replChain's ChoosePlayer arm: the lone as-enters
// opponent, or a chain head whose later links read the choice.
func (lc *lowerCtx) replChainChoosePlayer(ps *params, node, sa *cards.SA, other bool, cond *lCond) error {
	if node != sa && cond.Kind == cNone && isEnterChoosePlayer(node) &&
		len(lc.ab.Prog) > 0 && lc.ab.Prog[len(lc.ab.Prog)-1].Op == "ChooseColor" {
		// Jihad: a second as-enters choice after the colour. gorge poses both
		// with the spell still on the stack; the kernel's entry walker poses
		// one pre-entry choice, and its one chosen byte cannot hold the colour
		// and the opponent.
		return refuse("ETB replacement api ChoosePlayer after ChooseColor (a second as-enters choice)")
	}
	if !other && cond.Kind == cNone && node == sa && (spyglassChain(sa) || lookChooseTypeChain(sa) || lc.chainReadsEnterChosen(sa)) {
		// the chain head whose later links read the choice: the pod-wide
		// choice of an opponent, ahead of the look-then-name (or
		// look-then-type) ask or the counter/GainControl reader
		return lc.choosePlayerPod(ps)
	}
	// the lone as-enters opponent
	return lc.etbChoosePlayer(ps, node, sa, other, cond)
}

// replChainCaptiveControl is the ChangeZone body of Captive Audience: the
// chain head choosePlayerPod set the entering permanent's chosen player, and
// the body puts it under that player. The object is already entering, so the
// move is the control change alone (CR 110.2a), the op a GainControl$
// ChosenPlayer return lowers to.
func (lc *lowerCtx) replChainCaptiveControl(ps *params, node, sa *cards.SA, other bool, cond *lCond) bool {
	if other || sa.Sub != node || node.Sub != nil || cond.Kind != cNone ||
		!ps.is("Origin", "All") || !ps.is("Destination", "Battlefield") ||
		ps.str("Defined") != "ReplacedCard" || ps.str("GainControl") != "ChosenPlayer" {
		return false
	}
	ps.consume("Origin", "Destination", "Defined", "GainControl", "Hidden", "SpellDescription")
	lc.emit(lInstr{Op: "GainControl", Obj: lRef{Kind: rReplaced}, C: int(rChosenPlayer) << 8, CS: "dur=Permanent"})
	return true
}

// etbChoosePlayer lowers the as-enters opponent choice: the entering
// permanent's own, unconditional, sole ETB replacement part.
func (lc *lowerCtx) etbChoosePlayer(ps *params, node, sa *cards.SA, other bool, cond *lCond) error {
	if other || node != sa || cond.Kind != cNone || len(lc.ab.Prog) != 0 || !isEnterChoosePlayer(sa) {
		return refuse("ETB replacement api ChoosePlayer")
	}
	moved := 0
	for i := range lc.face.Repls {
		if lc.face.Repls[i].Event == "Moved" {
			moved++
		}
	}
	if moved != 1 {
		// two of the card's own entry replacements: gorge asks their CR
		// 616.1 order (a ChoosePlayer body never commutes there), which
		// the kernel does not pose (dirac StallChoosePlayerOrder)
		return refuse("ETB ChoosePlayer beside another entry replacement of the card")
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

// chainReadsEnterChosen reports whether a node after the as-enters
// ChoosePlayer reads the permanent's chosen player: ChosenCtrl in a param, or
// in the SVar body a param names (Canker Abomination's Count$Valid
// Creature.ChosenCtrl), or a GainControl$ ChosenPlayer (Captive Audience). The
// choice then has to be written on the permanent before the chain continues,
// so the head lowers through choosePlayerPod rather than the lone-node
// etbChoosePlayer.
func (lc *lowerCtx) chainReadsEnterChosen(sa *cards.SA) bool {
	reads := func(v string) bool {
		v = strings.TrimSpace(v)
		if v == "" {
			return false
		}
		if strings.Contains(v, "ChosenCtrl") || v == "ChosenPlayer" || v == "Player.Chosen" {
			return true
		}
		return lc.face != nil && strings.Contains(lc.face.SVars[v], "ChosenCtrl")
	}
	for n := sa.Sub; n != nil; n = n.Sub {
		for _, v := range n.Params {
			if reads(v) {
				return true
			}
		}
	}
	return false
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
