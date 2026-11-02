package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// "whenever one or more cards leave your graveyard" (ChangesZoneAll
// Origin$ Graveyard Destination$ Any). The trigger is a TrigLeaves line
// whose From is the graveyard: detach fires it for the card leaving, and
// the batch latch (Tf2Batch) collapses the cards one instruction moves
// together. The listener matches the card off the battlefield, as the
// discard event does (matchCardGen), with the listener's controller as
// the "you" of YouOwn.
//
//go:noinline
func (s *State) fireLeavesGraveyard(k *prep, o Obj, card CardID, owner uint8) {
	s.eachTrigger(k, kop.TrigLeaves, func(l trigListener, a *kop.Ability) {
		t := &a.Trig
		if t.From != kop.ZoneGraveyard {
			return
		}
		if t.Flags&kop.TfSelf != 0 {
			return
		}
		if t.Filter != 0 && !s.matchCardGen(k, t.Filter, card, owner, l.ctrl, o) {
			return
		}
		if t.Flags2&kop.Tf2YourTurn != 0 && s.active != l.ctrl || t.Flags2&kop.Tf2OppTurn != 0 && s.active == l.ctrl {
			return
		}
		if !s.trigOK(k, l, a) {
			return
		}
		if t.Flags2&kop.Tf2Batch != 0 && !s.batchTrigOK(l, a) {
			return
		}
		s.queueTrig(k, l, a, o, owner, 0)
	})
}

// enterFromGrave is enter for a card that left the graveyard. Gorge checks
// triggers after folding the move, so a permanent that returns itself from
// the graveyard (Skeleton Crew) sees its own graveyard-leave trigger from
// the battlefield; the detach walk above cannot, because the card is still
// in the graveyard then. The moved card is the only listener that can
// become active across this one move, so walk just its slot.
//
//go:noinline
func (s *State) enterFromGrave(k *prep, o Obj, ctrl uint8, tapped bool) Slot {
	return s.enter(k, o, ctrl, tapped, kop.ZoneGraveyard)
}

// fireLeavesGraveyardSelf queues the just-entered permanent's own
// Origin$ Graveyard Leaves triggers. It runs after enterNF, so the trigger
// zone (Battlefield) and the live card are what gorge's post-move check
// sees.
//
//go:noinline
func (s *State) fireLeavesGraveyardSelf(k *prep, sl Slot, o Obj) {
	p := &s.perms[sl]
	if p.obj != o {
		return
	}
	owner := s.objs[o].owner()
	card := s.objs[o].card
	s.slotTriggers(k, sl, func(a *kop.Ability) bool { return a.Trig.Event == kop.TrigLeaves }, func(l trigListener, a *kop.Ability) {
		t := &a.Trig
		if t.From != kop.ZoneGraveyard {
			return
		}
		self := l.sl == sl
		if t.Flags&kop.TfSelf != 0 {
			if !self {
				return
			}
		} else if t.Flags2&kop.Tf2SelfOrFilter == 0 || !self {
			if t.Filter != 0 && !s.matchCardGen(k, t.Filter, card, owner, l.ctrl, o) {
				return
			}
		}
		if t.Flags2&kop.Tf2YourTurn != 0 && s.active != l.ctrl || t.Flags2&kop.Tf2OppTurn != 0 && s.active == l.ctrl {
			return
		}
		if !s.trigOK(k, l, a) {
			return
		}
		if t.Flags2&kop.Tf2Batch != 0 && !s.batchTrigOK(l, a) {
			return
		}
		s.queueTrig(k, l, a, o, owner, 0)
	})
}
