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

// evolveArm marks a resolving Evolve trigger with its source's +1/+1 count
// before the body runs: trigAmt is -(count+1), a value no ETB trigger's
// amount takes (evolveDone reads it as the trigger finishes).
func (s *State) evolveArm(e *stackEntry, src *object) {
	// The source may have left the battlefield (or be a new object) while
	// the trigger waited; it still resolves (CR 603.4, 608.2b), but nothing
	// is counted, so it stays unarmed.
	if src.zone != kop.ZoneBattlefield || src.ver != e.srcVer {
		return
	}
	e.trigAmt = -int16(s.perms[src.slot].p1) - 1
}

// evolveDone fires the source's Evolved triggers (CR 702.100b; gorge
// resolveTop emits events.Evolved after the body when the source's +1/+1
// count rose) as an armed Evolve trigger finishes resolving. The source must
// still be the same object on the battlefield and its +1/+1 count must be
// above the armed one.
//
//go:noinline
func (s *State) evolveDone(k *prep, e *stackEntry) {
	src := &s.objs[e.obj]
	if src.zone != kop.ZoneBattlefield || src.ver != e.srcVer {
		return
	}
	if int(s.perms[src.slot].p1) <= int(-e.trigAmt)-1 {
		return
	}
	card := s.cardAt(src.slot)
	cd := &k.t.Cards[card]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbTriggered || a.Trig.Event != kop.TrigEvolved {
			continue
		}
		np := s.npend
		s.enqueue(e.obj, src.ver, card, uint8(i), s.ctrlOfObj(e.obj), e.obj, s.ctrlOfObj(e.obj), 0)
		s.panharmKeyword(k, src.slot, kop.TrigEvolved, e.obj, np)
	}
}
