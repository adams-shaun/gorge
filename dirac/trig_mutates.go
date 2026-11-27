package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Mode$ Mutates (CR 702.140f, the "whenever this creature mutates" /
// "whenever a creature you control mutates" triggers Kologran and Essence
// Symbiote print; gorge engine/rules/trigmatch/mutate.go). gorge publishes
// the merge as an events.Mutate whose Obj is the SURVIVING permanent — the
// mutated pile — and the matcher matches the trigger's ValidCard$ against
// that object, not against the trigger's source: the `Card.Self` shape (the
// pile IS the scanning source, the instance that mutated) and the lord
// shape (`Creature.YouCtrl`, where the scanning source is the lord and the
// event's Obj is the pile its controller owns) both read the pile. The
// kernel has no such event: TrigMutates is past nSubEvents (kw_gift.go) —
// no slot subscription — so resolveMutate's fire is a fireUnattached-style
// walk (trig_unattached.go) over the battlefield's own cards at the fold
// point, the event object being the pile slot: matchPerm runs the
// event-object filter exactly as mutatesMatches does, and the `you` of the
// filter reads the listener's controller, never the pile's, as gorge keeps
// the spec ctx's you the trigger's controller.
//
// The fold lands before the fire (gorge folds the Mutate event before
// triggers run), so the pile's top card, its components and its
// TimesMutated are already current when the ability is queued, and
// RefTriggerObj — gorge binds the event's Obj, triggerRemembered — is the
// pile: the Kologran family's Defined$ reads resolve to it.

// fireMutates queues the TrigMutates abilities that match the merge of the
// permanent o at slot sl under ctrl (trig_mutates.go; the pile itself is
// the event object, and its own TfSelf abilities hear it as the pile's
// card has just become the top).
//
//go:noinline
func (s *State) fireMutates(k *prep, sl Slot, o Obj, ctrl uint8) {
	if sl == NoSlot || !k.gen.anyMutateTrig {
		return
	}
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.occ, &ord)
	for _, ls := range ord[:n] {
		lp := &s.perms[ls]
		if lp.ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 {
			continue
		}
		rec := s.mergeRecOf(lp.obj)
		s.fireMutatesCard(k, sl, ls, s.cardAt(ls), o, ctrl)
		if rec != nil {
			// CR 702.140d: the pile has the abilities of every card beneath
			// its top (gorge MergedTriggerPush), in pile order
			for j := 0; j < int(rec.n); j++ {
				// an entry stores the object; its CardID names its face
				// (turnFace, face_multiface.go)
				s.fireMutatesCard(k, sl, ls, s.objs[Obj(rec.und[j]-1)].card, o, ctrl)
			}
		}
	}
}

// fireMutatesCard queues the TrigMutates abilities printed on component card
// c of the listener at ls, for the merge of the pile at sl (o under ctrl).
func (s *State) fireMutatesCard(k *prep, sl, ls Slot, c CardID, o Obj, ctrl uint8) {
	lp := &s.perms[ls]
	cd := &k.t.Cards[c]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbTriggered || a.Trig.Event != kop.TrigMutates {
			continue
		}
		if a.Trig.Flags&kop.TfSelf != 0 {
			if ls != sl {
				continue // Card.Self: the pile is the scanning source
			}
		} else if !s.matchPerm(k, a.Trig.Filter, sl, lp.ctrl, ls) {
			continue // the lord shape: the filter reads the pile, the you reads the listener
		}
		np := s.npend
		s.enqueue(lp.obj, s.objs[lp.obj].ver, c, uint8(i), lp.ctrl, o, ctrl, 0)
		s.panharmKeyword(k, ls, kop.TrigMutates, o, np)
	}
}

// prepMutateTrig sets anyMutateTrig: some program's trigger is TrigMutates
// (the fire's hot-path gate; kw_mutate.go's anyMutate gates the pile).
func prepMutateTrig(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigMutates {
				g.anyMutateTrig = true
				return
			}
		}
	}
}
