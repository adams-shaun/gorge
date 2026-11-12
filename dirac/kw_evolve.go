package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugEvolveNoRecheck skips the resolution recheck (the lockstep's mutation
// switch: only the fire-time gate is left).
var DebugEvolveNoRecheck bool

// evolveResolves rechecks Evolve's intervening "if" (CR 702.100a, 603.4) as
// the trigger resolves (trigResolveOK): the entering creature's power is
// still greater than the source's, or its toughness is. Two creatures
// entering together each queue an evolve, and the first counter makes the
// second's comparison fail. The entering creature (trigObj) is read live
// while it is the same object on the battlefield, else from the snapshot
// taken as the trigger was queued (li; CR 608.2h). A source that has left
// the battlefield has no counter to receive, so the ability is left to
// resolve to nothing. Gorge mirrors this in rules.evolveResolves.
func (s *State) evolveResolves(k *prep, e *stackEntry) bool {
	if DebugEvolveNoRecheck {
		return true
	}
	src := &s.objs[e.obj]
	if src.zone != kop.ZoneBattlefield || src.ver != e.srcVer {
		return true
	}
	l := s.lk(e.li)
	if e.li == 0 || l.obj != e.trigObj {
		return true // no snapshot (never for a queued ETB): fail open to the fire-time gate
	}
	p, t := int(l.power), int(l.tough)
	if o := &s.objs[e.trigObj]; o.zone == kop.ZoneBattlefield && o.ver == l.ver {
		p, t = s.pt(k, o.slot)
	}
	sp, st := s.pt(k, src.slot)
	return p > sp || t > st
}
