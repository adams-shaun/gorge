package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// "an imprinted card leaves exile" (Mode$ ChangesZone Origin$ Exile,
// Destination$ Any; Mirror Golem's forget line, Duplicant's imprinted-card
// cleanup line). gorge fires the line as the move completes and judges the
// ValidCard$ on the event's LKI snapshot (trigmatch/zone.go: a departure from
// exile reads the moving card as it existed there, CR 603.10's circumstances
// of the event — CR 607.2a's imprint association is exile-live, so a live read
// could never match). The kernel fires the same way from placeFromK, the sole
// card-placement choke point, BEFORE the placement sets the card's zone: the
// candidate is still in exile, so the ordinary gated store read
// (zone_imprint.go impStoreHas) answers gorge's LKI read. A destination of
// the battlefield keeps the same answer — gorge reads that candidate live,
// where the battlefield arm of the imprint association matches the same
// membership (engine/effects/context.go
// imprintAssociationContainsCandidate) — so the pre-move fire matches both.
// Only a ValidCard$ every clause of which is the imprint read lowers
// (kgen lower_changezonepairs.go czImprintOnly). No slot subscription
// (nSubEvents ends at TrigGiveGift): the firing walk scans the battlefield in
// gorge's listener order like the other no-subscription events (fireCrime).

// useLeaveExile is the table's genPrep.uses bit for a TrigLeaveExile trigger
// (genneed.go): the feature's fact stands in for a new genPrep bool.
var useLeaveExile = newGenUse(func(t *kop.Table) bool { return tableHasLeaveExile(t) })

// tableHasLeaveExile reports whether t carries a TrigLeaveExile trigger.
func tableHasLeaveExile(t *kop.Table) bool {
	for ci := range t.Cards {
		ab := t.Cards[ci].Abilities
		for ai := range ab {
			if a := &ab[ai]; a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigLeaveExile {
				return true
			}
		}
	}
	return false
}

// fireLeaveExile queues the TrigLeaveExile lines the card o's departure from
// exile satisfies. o is still in exile.
//
//go:noinline
func (s *State) fireLeaveExile(k *prep, o Obj) {
	if k == nil || k.gen == nil || !k.gen.uses.has(useLeaveExile) {
		return
	}
	ob := &s.objs[o]
	card, owner := ob.card, ob.owner()
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.occ, &ord)
	for _, sl := range ord[:n] {
		s.slotTriggers(k, sl, func(a *kop.Ability) bool { return a.Trig.Event == kop.TrigLeaveExile }, func(l trigListener, a *kop.Ability) {
			t := &a.Trig
			if t.Flags&kop.TfSelf != 0 {
				return
			}
			// The ValidCard$ is the listener's, not the resolving mover's: a
			// third effect moving an imprinted card out of exile is judged
			// against the carrier that holds the association. The move has
			// already detached the card, so name exile as its event-time zone.
			ctx := evalCtx{src: l.obj, srcSl: l.sl, ctrl: l.ctrl, origin: kop.ZoneExile}
			if t.Filter != 0 && !s.matchCardGenAt(k, t.Filter, card, owner, l.ctrl, o, &ctx) {
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
}
