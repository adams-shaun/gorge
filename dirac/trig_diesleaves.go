package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A creature's death is one ChangesZone event in gorge, whose triggers are
// walked listener by listener (rules forEachObject): each permanent's own
// triggers, Dies and Leaves lines together in ability order, and then the
// triggers granted to it (CR 603.3b, 613.7). The kernel fires the Dies
// event and then the Leaves one, each over all listeners, which lists a
// granted Dies trigger (Perigee Beckoner's "when this creature dies, return
// it") ahead of the host's own Leaves trigger (Syr Vondam's "dies ... while
// its power is 4 or greater"). When some granted trigger listens for Dies,
// the two events are walked as one.

// diesLeavesMerged reports whether fireLeave walks a creature's Dies and
// Leaves triggers as one pass: a granted Dies trigger exists, and nothing
// the merge does not model (a graveyard card's dies line) sits between the
// two events.
func diesLeavesMerged(k *prep) bool {
	g := k.gen
	return g != nil && g.anyLatentDies && !g.graveDies
}

// fireDiesLeaves enqueues the Dies and Leaves triggers of the creature at
// evSl that was put into a graveyard (czAmt the Leaves line's zone amount),
// then the emblems' Dies and Leaves triggers, as the two fireObj calls it
// replaces would.
//
//go:noinline
func (s *State) fireDiesLeaves(k *prep, evSl Slot, evObj Obj, evCtrl uint8, leaveAmt int16) {
	m := s.subs[kop.TrigDies].or(s.subs[kop.TrigLeaves]).and(s.occ)
	m = s.grantTrigHosts(k, m)
	if k.gen.anyBatch {
		s.g.batchLatch |= batchGrave // as fireLeavesBatched: the destination is a graveyard
	}
	var ord [MaxPerm]Slot
	n := s.listenerOrder(m, &ord)
	for _, sl := range ord[:n] {
		s.slotTriggers(k, sl, func(a *kop.Ability) bool {
			return a.Trig.Event == kop.TrigDies || a.Trig.Event == kop.TrigLeaves
		}, func(l trigListener, a *kop.Ability) {
			amt := int16(0)
			if a.Trig.Event == kop.TrigLeaves {
				amt = leaveAmt
			}
			s.objTrigGen(k, a.Trig.Event, l, a, evSl, evObj, evCtrl, amt)
		})
	}
	s.g.batchLatch &^= batchGrave
	for _, ev := range [2]kop.TrigEvent{kop.TrigDies, kop.TrigLeaves} {
		if k.gen.embObjEv&(1<<ev) != 0 {
			s.fireEmblemsObj(k, ev, evSl, evObj)
		}
	}
}
