package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The counter-threshold state trigger (Mode$ Always lowered over IsPresent$
// Card.Self+counters_GE<N>_<KIND>, Mazemind Tome; CR 603.8, gorge
// trigger_condition.go stateTriggerOutstanding): one instance at a time. A
// counter put on the source re-checks the threshold condition (objTrigGen's
// TrigCounterAddedOnce gate skips a put while an instance is queued or on
// the stack), and the re-check runs again at every resolution's finish
// (finishResolve) and after the end-of-turn sweep, so the source re-fires
// while it still meets the threshold. gorge re-checks on every event; for
// this shape only a counter put can newly satisfy the condition, and a
// countering resolution that then removes the source (Green Slime) makes
// its own leave moot by resolution's end -- re-checking inside the
// resolution (at counterAt) would queue an instance gorge's next-event
// check never does.

// DebugStateCtrNoLatch is the mutation switch: the outstanding scan sees
// nothing, so a put while an instance is queued or on the stack queues a
// second one. Read only on the gated state-trigger path, never in a walk
// without a state trigger.
var DebugStateCtrNoLatch bool

// DebugStateCtrNoRefire is the mutation switch: an instance's leave never
// re-checks, so a source still at threshold never re-fires after its
// instance is countered or swept. Read only on the gated path.
var DebugStateCtrNoRefire bool

// stateTrigOutstanding reports whether source o's ability ab already has an
// instance waiting in the pending queue or on the stack (gorge scans
// pendingTriggers for the source and index, then G.Stack for the source and
// ability).
//
//go:noinline
func (s *State) stateTrigOutstanding(o Obj, ab uint8) bool {
	if DebugStateCtrNoLatch {
		return false
	}
	for i := uint8(0); i < s.npend; i++ {
		if p := &s.pend[i]; p.obj == o && p.ab == ab {
			return true
		}
	}
	for i := 0; i < int(s.sp); i++ {
		if e := &s.stack[i]; e.kind == seTrigger && e.obj == o && e.ab == ab {
			return true
		}
	}
	return false
}

// stateCtrRefire re-checks every battlefield state trigger's condition (in
// gorge's walk order, as its check at the next event after an instance
// leaves the stack) and queues the sources that still meet it with nothing
// outstanding.
//
//go:noinline
func (s *State) stateCtrRefire(k *prep) {
	if DebugStateCtrNoRefire {
		return
	}
	s.eachTrigger(k, kop.TrigCounterAddedOnce, func(l trigListener, a *kop.Ability) {
		if a.Trig.Flags4&kop.Tf4StateCtr == 0 {
			return
		}
		if s.stateTrigOutstanding(l.obj, l.ab) {
			return
		}
		if !s.trigOK(k, l, a) {
			return
		}
		s.queueTrig(k, l, a, l.obj, l.ctrl, 0)
	})
}
