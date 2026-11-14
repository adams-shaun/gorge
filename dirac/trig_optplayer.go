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
//
// With Tf5OptSrcCtrl (a DamageDone line's OptionalDecider$
// TriggeredSourceController) it is the controller of the entry's trigger
// object, the damage source, read live at the ask (a source that left
// reads its owner); gorge deciderFromSpecTriggeredSourceController.
func (s *State) optTrigAsker(e *stackEntry, f4 kop.TrigFlags4, f5 kop.TrigFlags5) uint8 {
	if DebugOptDeciderCtrl {
		return e.ctrl
	}
	if f5&kop.Tf5OptSrcCtrl != 0 && e.trigObj != NoObj {
		return s.ctrlOfObj(e.trigObj)
	}
	if f4&kop.Tf4OptTrigPlayer != 0 {
		return e.trigPl
	}
	return e.ctrl
}
