package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// "Whenever a creature card is put into a graveyard from a library"
// (Dreadhound; Mode$ ChangesZone Origin$ Library Destination$ Graveyard).
// gorge matches ValidCard$ against each card such a move puts into a
// graveyard (trigmatch ZoneChangeMatchesWithCapture), one trigger per card in
// move order. The line is a TrigToGraveyard ability with From ZoneLibrary,
// fired where placeFromK puts the card, after any replacement chose its zone
// (Rest in Peace's exile and Progenitus' shuffle do not fire it). Only
// battlefield (and command zone) listeners hear it: kgen refuses the card's
// own line (Narcomoeba), which listens from the graveyard it lands in.

// fireLibToGrave queues the TrigToGraveyard lines o's move from a library to
// a graveyard satisfies. o is already in the graveyard; the listener matches
// the card as the discard event does (matchCardGen), with the listener's
// controller as the "you" of YouOwn.
//
//go:noinline
func (s *State) fireLibToGrave(k *prep, o Obj) {
	ob := &s.objs[o]
	card, owner := ob.card, ob.owner()
	s.eachTrigger(k, kop.TrigToGraveyard, func(l trigListener, a *kop.Ability) {
		t := &a.Trig
		if t.From != kop.ZoneLibrary || t.Flags&kop.TfSelf != 0 {
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
