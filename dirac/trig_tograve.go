package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// "a creature card is put into a graveyard from anywhere other than the
// battlefield" (ChangesZone with an Origin$ set that leaves the battlefield
// out: Syr Konrad, the Grim, Ultron the Annihilator, Ultron's Auxiliary). The
// line is one TrigToGraveyard ability whose Trigger.N carries the origin
// zones as a 1<<kop.Zone mask (0 = any), mirroring TrigLandPlayed's origin
// mask. The battlefield origin is not part of the family: a Dies line and a
// ToGraveyard line never share a move, so dies triggers keep their own event
// and their own firing (fireDies).
//
// gorge fires the line as the move completes (trigmatch
// ZoneChangeMatchesWithCapture reads the move's from-zone against Origin$ and
// its to-zone against Destination$), from the listener that is on the
// battlefield at that moment. The kernel fires the same way in placeFromK,
// the sole card-placement choke point: o is already in its graveyard, its
// from-zone rides the call.

// fireToGrave queues the TrigToGraveyard lines that o's move from `from` into
// a graveyard satisfies. o is already placed; the listener matches the card
// as the discard event does (matchCardGen, the filter compiled in the
// graveyard), with the listener's controller as the "you" of YouOwn.
//
//go:noinline
func (s *State) fireToGrave(k *prep, o Obj, from kop.Zone) {
	ob := &s.objs[o]
	card, owner := ob.card, ob.owner()
	s.eachTrigger(k, kop.TrigToGraveyard, func(l trigListener, a *kop.Ability) {
		t := &a.Trig
		if t.N != 0 && t.N&(1<<uint(from)) == 0 {
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
		s.queueTrig(k, l, a, o, owner, 0)
	})
}
