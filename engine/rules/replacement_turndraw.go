package rules

import "github.com/adams-shaun/gorge/engine/decision"

// The draw step's turn-based draw (CR 504.1) can park on a replacement
// choice: a lone Optional$ True Draw replacement's apply / decline
// (Obstinate Familiar, replChoiceDraw) or a CR 616.1 order. drawStepTurnAction
// then returns without the step's own Priority emit (CR 405.1: priority
// comes after the turn-based action completes), and the answer must grant it.
// A Dredge ask's answer does, through the resolution resume path; a
// KReplacement answer runs handleReplacement, which grants nothing, so the
// draw step went on with the pass state the upkeep left -- the non-active
// player holding priority with the active player's pass counted -- and the
// active player never received priority in their own draw step (CR 117.3a).
// The choice the turn draw parked is marked turnDraw, and its answer grants
// the active player priority once the draw has finished.

// markTurnDrawChoice marks the front parked replacement choice rcs[0] as the
// turn draw's when the draw left a KReplacement ask pending.
func markTurnDrawChoice(pending *decision.Decision, rcs []replChoice) {
	if pending != nil && pending.Kind == decision.KReplacement && len(rcs) > 0 {
		rcs[0].turnDraw = true
	}
}

// turnDrawPriorityDue reports, after an answered replacement choice, whether
// the draw step's priority is due now: the answered choice was the turn
// draw's (turnDraw, read before the answer), the game goes on, and nothing
// else is left to ask (a further parked choice of the same draw carries the
// mark on through markTurnDrawChoice).
func turnDrawPriorityDue(turnDraw, over bool, pending *decision.Decision) bool {
	return turnDraw && !over && pending == nil
}
