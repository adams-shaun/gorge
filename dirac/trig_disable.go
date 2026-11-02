package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DisableTriggers for entering objects (CR 603.2g):
// gorge disableTriggersExcludes runs on every trigger line, printed or
// granted, after the zone gate, so an enters event whose object matches an
// active kop.StatDisableETB static (ValidCause$) queues nothing: not the
// battlefield listeners, not the graveyard ones, not an emblem's. A source
// that lost its abilities stops applying (eachStatic skips genLost, as
// gorge's activeStatics does). An event with no single object never matches
// (evSl == NoSlot, gorge ev.Obj == 0).

// DebugDisETBOff is the lockstep mutation: no enters is suppressed, so the
// kernel queues the triggers gorge drops.
var DebugDisETBOff bool

// etbDisabled: the object at evSl entering the battlefield triggers nothing.
func (s *State) etbDisabled(k *prep, evSl Slot) bool {
	return !DebugDisETBOff && evSl != NoSlot && s.staticFlagGen(k, kop.StatDisableETB, evSl)
}
