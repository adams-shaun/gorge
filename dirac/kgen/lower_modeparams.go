package kgen

import "strings"

// lModeOperand is what modeParam reads out of one mode param: the player
// scope a player-event trigger gates with (Whose).
type lModeOperand struct {
	Who uint8
}

// modeParam is the one reader of a mode param by param NAME, shared by every
// mode of a kind (the T17 design): a mode lowering calls it for a param it
// does not already read, and a mode that reads the param itself is untouched
// (the additive rule). kind is "trig", "stat" or "repl"; only the trigger
// kind has arms so far.
//
// ValidPlayer (a player-event trigger: Drawn). gorge matches the whole
// grammar of MatchesPlayerSpec against the event's player relative to the
// trigger's controller; the kernel's player-trigger walk (playerTrigGen)
// gates a listener with Whose (you / opponent / any). What lowers is exactly
// what those express (whoOfPlayer): You, Opponent, Player.Opponent,
// Player.Other, Player and the empty value. Every other qualifier refuses,
// Player.Active and the NonActive spellings among them: "the active player"
// is relative to the turn, not to the listener's controller, and no lockstep
// card yet vouches for the two-seat your-turn reading of NonActive.
func (lc *lowerCtx) modeParam(kind, mode, param string, ps *params) (lModeOperand, error) {
	if kind != "trig" {
		return lModeOperand{}, refuse("%s %s mode param %s$ has no shared reader", kind, mode, param)
	}
	switch param {
	case "ValidPlayer":
		v := strings.TrimSpace(ps.str(param))
		if w, ok := whoOfPlayer(v); ok {
			return lModeOperand{Who: w}, nil
		}
		return lModeOperand{}, refuse("%s ValidPlayer %q", mode, v)
	}
	return lModeOperand{}, refuse("%s %s mode param %s$ has no shared reader", kind, mode, param)
}

// drawnScope lowers the "which player drew" scope of a Mode$ Drawn line
// (gorge drawnMatches, trigmatch/cards.go): ValidCard$ is matched against the
// drawn card, whose owner is the drawing player (a card is drawn from its
// owner's library into its owner's hand, CR 108.4a), and ValidPlayer$ against
// the drawing player. A ValidCard$ naming only the owner side is a player
// scope; a ValidCard$ that tests the card itself (a type, a name) is not
// modelled, as the kernel's draw event carries no card to test.
func (lc *lowerCtx) drawnScope(tp *params, tr *lTrigger) error {
	vc := tp.str("ValidCard")
	switch vc {
	case "Card.YouCtrl", "Card.YouOwn":
		tr.Whose = whoYou
	case "Card.OppOwn", "Card.OppCtrl":
		// a card being drawn is in its owner's hand, whose controller
		// is its owner (CR 108.4a; gorge matches OppCtrl on the drawn
		// object's Controller, its owner's seat): Sheoldred
		tr.Whose = whoOpp
	case "Card":
		// every draw (Spiteful Visions, Phyrexian Tyranny); no ValidPlayer$
		// rides beside it
		if vp := tp.str("ValidPlayer"); vp != "" {
			return refuse("Drawn ValidCard %q ValidPlayer %q", vc, vp)
		}
		tr.Whose = whoAny
	case "":
		// ValidPlayer$-scoped draws: the drawing player relative to the
		// controller; Number$ counts the drawer's own draws, as nthDrawOK does
		var op lModeOperand
		var err error
		if tp.str("ValidPlayer") != "" { // neither param: not a scope, refused
			op, err = lc.modeParam("trig", "Drawn", "ValidPlayer", tp)
		} else {
			err = refuse("no scope")
		}
		if err != nil {
			return refuse("Drawn ValidCard %q ValidPlayer %q", vc, tp.str("ValidPlayer"))
		}
		tr.Whose = op.Who
	default:
		return refuse("Drawn ValidCard %q", vc)
	}
	return nil
}
