package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// "whenever one or more cards are put into exile from graveyards and/or the
// battlefield" (ChangesZoneAll Origin$ Battlefield,Graveyard Destination$
// Exile; Ketramose, the New Dawn). gorge matches each move against the line's
// Origin$ and Destination$ sets (trigmatch ZoneChangeMatchesWithCapture) and,
// in an open zone batch, queues the line once. The line is one TrigLeaves
// ability with a destination mask (Trigger.N) and kop.Tf2LeavesGrave: the
// battlefield side fires from fireObj like any destination-mask line; the
// graveyard side fires here, where the card is placed and its destination is
// known, with the same batch latch (the pending-entry scan keys on the
// ability, so both sides share one instance per batch).

// fireLeavesGraveTo queues the Tf2LeavesGrave lines that o's move from a
// graveyard to z satisfies. o is already placed in z; the listener matches
// the card as the discard event does (matchCardGen), with the listener's
// controller as the "you" of YouOwn.
//
//go:noinline
func (s *State) fireLeavesGraveTo(k *prep, o Obj, z kop.Zone) {
	ob := &s.objs[o]
	card, owner := ob.card, ob.owner()
	s.eachTrigger(k, kop.TrigLeaves, func(l trigListener, a *kop.Ability) {
		t := &a.Trig
		if t.Flags2&kop.Tf2LeavesGrave == 0 || t.N&(1<<uint(z)) == 0 {
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
		if t.Flags2&kop.Tf2Batch != 0 && !s.batchTrigOK(l, a, o) {
			return
		}
		s.queueTrig(k, l, a, o, owner, 0)
	})
}
