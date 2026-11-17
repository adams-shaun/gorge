package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A prefix script's tap that queues a trigger another player controls and
// that poses a build ask -- Relic Bind on a Talisman of Unity: a mode the
// Aura's Taps watcher asks its controller when the Talisman taps -- ends the
// line: gorge's search (potential_script.go visit) answers only the payer's
// own asks, and at any other player's non-priority ask it returns false, so
// no continuation of the prefix is offered the play (CR 603.3c, 605.1a). A
// may-election (KTriggerOptional: Judge of Currents' "you may gain 1 life")
// is a decision too, but it is posed only when the trigger resolves (CR
// 603.5), so it ends the line only where the search reaches it: a play that
// waits for the stack (waitsForStack) branches on the pass that lets the
// trigger resolve and hits the foreign election there; an instant-speed play
// is offered at the payer's priority with the trigger still on the stack, and
// the election is the opponent's to answer later. A foreign trigger that asks
// nothing is placed and the search goes on.

// foreignPendAsks reports whether a queued trigger controlled by a player
// other than p poses an ask gorge's search would have to answer before the
// play is offered: a build ask (targets, modes) is posed as the trigger is
// placed, so it ends the line whatever the play's timing; a may-election
// (kop.TfOptional, CR 603.5) is posed only at resolution, so it ends the line
// only for a play that waits for the stack (resolve, playNeedsEmptyStack).
func (s *State) foreignPendAsks(k *prep, p uint8, resolve bool) bool {
	for j := 0; j < int(s.npend); j++ {
		if s.pend[j].ctrl == p {
			continue
		}
		a := s.pendAbility(k, j)
		if a == nil {
			continue
		}
		if a.NTargets != 0 || a.NModes != 0 {
			return true
		}
		if resolve && a.Trig.Flags&kop.TfOptional != 0 && s.pendOptAsker(j, a) != p {
			return true
		}
	}
	return false
}

// pendOptAsker is the seat pending trigger j's may-election (kop.TfOptional)
// is posed to once it is on the stack: optTrigAsker's answer -- the
// controller, or with Tf4OptTrigPlayer the trigger's player, or with
// Tf5OptSrcCtrl the live controller of the trigger's object -- not the
// controller alone, which Tf4/Tf5 can override in the payer's favour. The
// entry fields are the ones takePending copies onto the stack entry, so the
// predicate reads the seat the resolution ask will carry. A trigger object
// whose controller changes between the tap and the resolution (a Control
// Magic taken while the play resolved) is read live at the ask and only
// approximated here.
func (s *State) pendOptAsker(j int, a *kop.Ability) uint8 {
	pt := &s.pend[j]
	e := stackEntry{ctrl: pt.ctrl, trigObj: pt.trigObj, trigPl: pt.trigPl}
	return s.optTrigAsker(&e, a.Trig.Flags4, a.Trig.Flags5) // trig_optplayer.go
}
