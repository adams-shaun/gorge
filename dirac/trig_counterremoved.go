package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// "Whenever counters are removed from CARDNAME" (gorge Mode$ CounterRemoved /
// CounterRemovedOnce, trigmatch counterRemovedMatches: a CounterChange with a
// negative Amount on the trigger's own card, CounterType$ the kind). Only the
// Card.Self shapes lower, so the listener is the object that lost the counters
// and there is no slot subscription (the events sit past nSubEvents).
//
//   - TrigCounterRemovedOnce: one trigger per removal batch (one kind off one
//     object), the amount the batch size (Chandra, Fire Artisan's "that much
//     damage").
//   - TrigCounterRemoved with Tf4PerCounter: a plain "whenever a counter is
//     removed" line, one trigger per counter removed, each carrying amount 1
//     (CR 603.2c; gorge CounterRemovedInstances).
//   - TrigCounterRemoved without it: NewCounterAmount$ 0, "the last counter is
//     removed": one trigger when the kind is left at none, the amount the
//     batch size.
//
// The battlefield removal sites that fire it: OpRemoveCounter,
// OpRemoveCounterRem, OpClearCounters, OpRemoveCounterAll, AddOrRemoveCounter,
// MoveCounter, the RemoveAnyCounter election (ctr_removeany.go rcaFinish, one
// batched removal per kind, gorge removeCounterAnyCarrier), the OpSupport
// remove pick (ctr_removechoices.go rmctrTake), the SubCounter cost (the
// announced X, the fixed pick, the any-kind pick, the choice of a counted
// creature's counter), a loyalty ability's cost, a mana ability's counter cost
// (ctrGapDo, ctrDeadGap, storeGap and riderGap) and the stun counter the untap
// replacement spends instead of untapping (turn.go, repl_untapstep.go,
// untapSlot, untapAll, untapCost, untapOthers, gainControlOne and
// OpGainControlEOT) and the shield counter spent instead of damage or
// destruction (spendShield, CR 122.1c; gorge emits a CounterChange -1 there
// too). A removal by damage or +1/+1 / -1/-1 annihilation fires nothing (kgen refuses +1/+1 and -1/-1 lines for it;
// gorge folds a walker's damage loyalty loss into the Damage event, never a
// CounterChange). A suspended card's upkeep time counter fires from exile
// (resolveSuspendTick, fireCtrRemovedExile).

// useCounterRemoved is the table's genPrep.uses bit for a CounterRemoved or
// CounterRemovedOnce trigger (genneed.go).
var useCounterRemoved = newGenUse(func(t *kop.Table) bool {
	for ci := range t.Cards {
		ab := t.Cards[ci].Abilities
		for ai := range ab {
			if a := &ab[ai]; a.Kind == kop.AbTriggered && (a.Trig.Event == kop.TrigCounterRemoved || a.Trig.Event == kop.TrigCounterRemovedOnce) {
				return true
			}
		}
	}
	return false
})

// subCtrFire removes up to n counters of kind c from the permanent at sl and
// fires the removal event for the number that left.
func (s *State) subCtrFire(k *prep, sl Slot, c, n int) {
	p := &s.perms[sl]
	had := p.ctrN(s, c)
	p.subCtrN(s, c, n)
	s.fireCtrRemoved(k, sl, c, min(n, had))
}

// fireCtrRemoved queues the removal lines of the permanent at sl, which just
// lost n counters of kind c (the count left is read from it).
//
//go:noinline
func (s *State) fireCtrRemoved(k *prep, sl Slot, c, n int) {
	if n <= 0 || k == nil || k.gen == nil || !k.gen.uses.has(useCounterRemoved) {
		return
	}
	left := s.perms[sl].ctrN(s, c)
	isEv := func(a *kop.Ability) bool {
		return a.Trig.Event == kop.TrigCounterRemoved || a.Trig.Event == kop.TrigCounterRemovedOnce
	}
	s.slotTriggers(k, sl, isEv, func(l trigListener, a *kop.Ability) {
		s.queueCtrRemoved(k, l, a, c, n, left)
	})
}

// fireCtrRemovedExile queues the TriggerZones$ Exile removal lines of the
// exiled card o, which just lost n counters of kind c and has left counters.
//
//go:noinline
func (s *State) fireCtrRemovedExile(k *prep, o Obj, c, n, left int) {
	if n <= 0 || k == nil || k.gen == nil || !k.gen.uses.has(useCounterRemoved) {
		return
	}
	ob := &s.objs[o]
	cd := &k.t.Cards[ob.card]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbTriggered || a.AFlags&kop.AfLatent != 0 || a.Trig.Zone != kop.ZoneExile ||
			a.Trig.Event != kop.TrigCounterRemoved && a.Trig.Event != kop.TrigCounterRemovedOnce {
			continue
		}
		l := trigListener{obj: o, sl: NoSlot, ctrl: ob.owner(), card: ob.card, ab: uint8(i)}
		s.queueCtrRemoved(k, l, a, c, n, left)
	}
}

// queueCtrRemoved queues listener l's line a for a removal of n counters of
// kind c that left left of them, per the shapes above.
func (s *State) queueCtrRemoved(k *prep, l trigListener, a *kop.Ability, c, n, left int) {
	t := &a.Trig
	if int(t.N) != c || !s.trigOK(k, l, a) {
		return
	}
	switch {
	case t.Event == kop.TrigCounterRemoved && t.Flags4&kop.Tf4PerCounter != 0:
		for i := 0; i < n; i++ {
			s.queueTrig(k, l, a, l.obj, l.ctrl, 1)
		}
		return
	case t.Event == kop.TrigCounterRemoved && left != 0:
		return // NewCounterAmount$ 0: only the removal that empties the kind
	}
	s.queueTrig(k, l, a, l.obj, l.ctrl, int16(n))
}

// clearCtrFire is subCtrFire for a clear: kind c on the permanent at sl is set
// to none (OpClearCounters), firing the removal of what it held.
func (s *State) clearCtrFire(k *prep, sl Slot, c int) {
	p := &s.perms[sl]
	had := p.ctrN(s, c)
	p.setCtrN(s, c, 0)
	s.fireCtrRemoved(k, sl, c, had)
}
