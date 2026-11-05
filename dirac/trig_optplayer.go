package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugOptDeciderCtrl is a mutation switch for the lockstep tests: the
// controller answers every optional trigger, as before OptionalDecider$
// TriggeredPlayer lowered.
var DebugOptDeciderCtrl bool

// optTrigAsker is the seat an optional trigger's resolution ask (CR 603.5) is
// posed to: the controller, or with Tf4OptTrigPlayer the trigger's player
// (a Phase trigger's active player, gorge deciderFromSpecTriggeredPlayer;
// Peace in Our Time). No State byte: the player rides the entry's trigPl.
func optTrigAsker(e *stackEntry, f4 kop.TrigFlags4) uint8 {
	if f4&kop.Tf4OptTrigPlayer != 0 && !DebugOptDeciderCtrl {
		return e.trigPl
	}
	return e.ctrl
}
