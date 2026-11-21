package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The trigger player of a kop.TrigNSrcCtrl BecomesTarget watcher (Thunderbreak
// Regent's "that player", Defined$ TriggeredSourceController) is read again as
// the trigger resolves, as gorge reads it: effects.TriggeredSourceController
// takes the controller of the source the TargetsChosen event captured
// (TriggerSource: the targeting spell itself, or an ability's source object,
// rules.protectionSource) at resolution, not at the targeting. That object
// keeps its id in every zone, so gorge answers:
//   - a spell still on the stack: the spell's controller now;
//   - a permanent on the battlefield (the resolved spell, or the ability's
//     source): its controller now;
//   - anywhere else (a countered spell in its graveyard, a sacrificed source):
//     its owner, the controller every move off the battlefield or the stack
//     resets to (events apply_zone.go).
// It is not last-known information (CR 608.2h, 113.7a): a stolen Mogg Fanatic
// sacrificed to target the Regent names its owner, not the player who
// sacrificed it. The kernel holds the same object in pendingTrig.trigSA /
// stackEntry.trigSA, so it costs 0 bytes: the queue-time read
// (btTriggerPlayer) stays the ask-time value and resolveTop overwrites the
// resolving copy.

// DebugBtSrcCtrlGone is the lockstep mutation of the resolution-time read: a
// targeting object that has left the stack and the battlefield names the
// listener's controller instead of its owner, so
// TestFourSeatThunderbreakCounteredLockstep must diverge. Never set in
// production.
var DebugBtSrcCtrlGone bool

// btSrcCtrlLive sets the resolving entry's trigger player for a
// TrigNSrcCtrl line from the targeting object's live controller.
//
//go:noinline
func (s *State) btSrcCtrlLive(k *prep, e *stackEntry) {
	if e.kind != seTrigger || DebugSrcCtrlListener {
		return
	}
	cd := &k.t.Cards[e.card]
	if int(e.ab) >= len(cd.Abilities) || cd.Abilities[e.ab].Trig.N != kop.TrigNSrcCtrl {
		return
	}
	o := Obj(e.saWord() - 1) // code-ok: trigSA stores the object + 1
	pl := s.liveCtrlOf(o)
	gone := s.objs[o].zone != kop.ZoneStack && s.objs[o].zone != kop.ZoneBattlefield
	if gone && DebugBtSrcCtrlGone {
		pl = e.ctrl
	}
	if st := DebugBtStats; st != nil {
		st.SrcCtrlReads++
		if gone {
			st.SrcGone++
		}
	}
	s.res.e.trigPl = pl
}

// liveCtrlOf is gorge's state.Object.Controller for object o: a spell's
// stack controller, a permanent's controller, else its owner.
func (s *State) liveCtrlOf(o Obj) uint8 {
	if s.objs[o].zone == kop.ZoneStack {
		if se := s.spellEntryOf(o); se != nil {
			return se.ctrl
		}
	}
	return s.ctrlOfObj(o)
}

// trigSAReferenced reports whether a stack entry or pending trigger still
// names o as its BecomesTarget targeting object (trigSA), so the token table
// does not hand o's id to a new token before the trigger resolves.
func (s *State) trigSAReferenced(o Obj) bool {
	w := objWord(o + 1) // code-ok: trigSA stores the object + 1
	for i := 0; i < int(s.sp); i++ {
		if s.stack[i].saWord() == w {
			return true
		}
	}
	for i := 0; i < int(s.npend); i++ {
		if s.pend[i].saWord() == w {
			return true
		}
	}
	return s.res.active && s.res.e.saWord() == w
}

// btSrcCtrlRes is resolveTop's gate for btSrcCtrlLive: a BecomesTarget
// trigger (trigSA set) on a table with a watcher.
func (s *State) btSrcCtrlRes(k *prep, e *stackEntry) {
	if e.saWord() != 0 && k.gen != nil && k.gen.btWatch {
		s.btSrcCtrlLive(k, e)
	}
}
