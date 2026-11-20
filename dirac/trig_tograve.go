package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// "a creature card is put into a graveyard from anywhere other than the
// battlefield" (ChangesZone with an Origin$ set that leaves the battlefield
// out: Syr Konrad, the Grim, Ultron the Annihilator, Ultron's Auxiliary) and
// "a creature card is put into a graveyard from a library" (Dreadhound;
// Origin$ Library Destination$ Graveyard). The first is one TrigToGraveyard
// ability whose Trigger.N carries the origin zones as a 1<<kop.Zone mask
// (0 = any), mirroring TrigLandPlayed's origin mask; the second is the same
// event with From ZoneLibrary. The battlefield origin is not part of the
// family: a Dies line and a ToGraveyard line never share a move, so dies
// triggers keep their own event and their own firing (fireDies).
//
// gorge fires the line as the move completes (trigmatch
// ZoneChangeMatchesWithCapture reads the move's from-zone against Origin$ and
// its to-zone against Destination$, one trigger per card in move order), from
// the listener that is on the battlefield at that moment. The kernel fires the
// same way in placeFromK, the sole card-placement choke point: o is already in
// its graveyard, after any replacement chose its zone (Rest in Peace's exile
// and Progenitus' shuffle do not fire it), its from-zone rides the call. Only
// battlefield (and command zone) listeners hear it: kgen refuses the card's
// own line (Narcomoeba), which listens from the graveyard it lands in.

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
		if t.From != kop.ZoneNone && t.From != from || t.Flags&kop.TfSelf != 0 {
			return
		}
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
		if !s.batchTrigOK(l, a, o) {
			return // a "one or more" line already queued in the open batch
		}
		s.queueTrig(k, l, a, o, owner, 0)
	})
}

// fireToGraveDies queues the Dies halves of split "whenever a creature card
// is put into an opponent's graveyard from anywhere" lines (Tf2ToGraveHalf)
// for the permanent at sl headed into z. It runs from the removal point
// (removePerm's top), where the permanent is still on the battlefield with
// every characteristic the dies event reads, and the live subscriptions are
// exactly gorge's per-move live walk: the listeners the sweep removed before
// this move are already unsubscribed, so a from-anywhere watcher destroyed by
// the same wipe fires only for the deaths that complete before its own
// (CR 603.6c: the spelling is never a leaves-the-battlefield ability, so it
// has none of CR 603.10a's look-back; gorge trigger_match.go's split gate
// keeps the printed line in the live pass). The Dies walks skip the marked
// half, so this is its only firing site. z is the removal's post-dieZone
// destination: a death replaced to exile or the library fires nothing, the
// same gate placeFromK puts on fireToGrave.
//
//go:noinline
func (s *State) fireToGraveDies(k *prep, sl Slot, z kop.Zone) {
	if k.gen == nil || !k.gen.anyToGrave || z != kop.ZoneGraveyard || !s.subs[kop.TrigDies].any() {
		return
	}
	p := &s.perms[sl]
	o, ctrl := p.obj, p.ctrl
	if !s.isCreature(k, sl) && !k.gen.anyDiesNC {
		return // as fireLeave's Dies arm: the filters decide from here
	}
	s.eachBattlefieldTrigger(k, kop.TrigDies, func(l trigListener, a *kop.Ability) {
		if a.Trig.Flags2&kop.Tf2ToGraveHalf == 0 || l.sl == sl {
			return
		}
		s.objTrigGen(k, kop.TrigDies, l, a, sl, o, ctrl, 0)
	})
}
