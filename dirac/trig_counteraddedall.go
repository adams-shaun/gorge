package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Mode$ CounterAddedAll, "whenever you put one or more counters on one or
// more permanents" (gorge trigmatch counterAddedAllMatches and the shared
// action bracket in trigger_match.go / trigger_counter_batch.go). The
// per-recipient TrigCounterAddedOnce events are fired one per recipient; the
// watcher is one trigger per counter-adding action, its amount the total the
// action placed on matching recipients (so Bioessence Hydra's "put that many
// +1/+1 counters" sees the whole batch).
//
// The batch mirrors gorge's action bracket exactly: ctrAllBatchOpen is set
// only where gorge's effPutCounter/effPutCounterAll open beginActionBatch --
// the multi-recipient PutCounter loops, the attacker-group put and the
// Support$ pick loop. A MultiplyCounter sweep does NOT bracket (gorge's
// effMultiplyCounter emits each recipient's CounterChange outside the
// bracket), and neither do entry counters, so those fire per recipient. A
// single-recipient site is a batch of one. No slot subscription: the event
// ordinal is past nSubEvents, so the walk scans the battlefield, as
// DiscardedAll does, and adds the hosts of a Continuous static that grants
// the trigger. The subscription alias in the trigMask builders exists only
// so the existing counter sites' cheap subs[TrigCounterAddedOnce] gate still
// calls fireObj, whose hook runs the watcher.

// grantCtrAllUse: a card's Continuous static grants a CounterAddedAll
// triggered ability. The watcher's ordinal is past grantTrigEv's 32 bits, so
// the table carries this flag and ctrAllNote walks the granted hosts. An
// emblem's grant is marked by prepEmbGrant when prepareGen sees it.
var grantCtrAllUse = newGenUse(func(t *kop.Table) bool {
	for i := range t.Cards {
		c := &t.Cards[i]
		for j := range c.Abilities {
			a := &c.Abilities[j]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && a.Static.Grant != 0 &&
				int(a.Static.Grant) <= len(c.Abilities) {
				if ga := &c.Abilities[a.Static.Grant-1]; ga.Kind == kop.AbTriggered && ga.Trig.Event == kop.TrigCounterAddedAll {
					return true
				}
			}
		}
	}
	return false
})

// DebugCtrAllPerRecipient queues the watcher once per recipient instead of
// once per batch (mutation testing only: TestCounterAddedAllJoin flips it).
var DebugCtrAllPerRecipient bool

// DebugCtrAllFires counts the watcher events that reached a listener (test
// evidence only; never read by the kernel).
var DebugCtrAllFires int

// DebugCtrAllJoins counts events that joined an instance already queued for
// the batch (test evidence only; never read by the kernel).
var DebugCtrAllJoins int

// ctrAllNote fires the CounterAddedAll watchers for the counters a counter
// site just placed on the permanent at evSl. It runs from fireObj's
// TrigCounterAddedOnce hook, so every counter-adding site gets it without a
// second call beside each fireObj.
//
//go:noinline
func (s *State) ctrAllNote(k *prep, evSl Slot, evObj Obj, evCtrl uint8, amt int16) {
	if k.gen == nil || amt <= 0 || evSl == NoSlot {
		return
	}
	// The Once subscription is the gate: a CounterAddedAll card (or grant)
	// subscribes its slot to TrigCounterAddedOnce (prep.go, gen.go,
	// chars_gen.go), so a table without one pays a mask test per placement.
	m := s.subs[kop.TrigCounterAddedOnce].and(s.occ)
	if k.gen.uses.has(grantCtrAllUse) {
		// a Continuous static granting the trigger to another permanent
		// (Aura/Equipment AddTrigger$, grantedAbilitiesN): its host is a
		// listener without a once-subscription (eachBattlefieldTrigger's
		// grant walk; triggers_gen.go). The event's ordinal (86) is past
		// grantTrigEv's 32 bits, so the table carries a flag instead.
		m = s.grantTrigHosts(k, m)
	}
	if k.gen.gainsCtrAll {
		// a GainsTrig recipient (Koh) lends a CounterAddedAll line from a
		// foreign zone; its slot is no subscriber, so add it here (the
		// gainsTrigEv Once alias arms the counter sites' gate above)
		m = s.gainsTrigHosts(k, m)
	}
	if !m.any() {
		return
	}
	var ord [MaxPerm]Slot
	n := s.listenerOrder(m, &ord)
	for _, sl := range ord[:n] {
		s.slotTriggers(k, sl, func(a *kop.Ability) bool { return a.Trig.Event == kop.TrigCounterAddedAll },
			func(l trigListener, a *kop.Ability) {
				s.objTrigGen(k, kop.TrigCounterAddedAll, l, a, evSl, evObj, evCtrl, amt)
			})
	}
}

// ctrAllBatchOpen marks the pending queue's start of one counter-adding
// action and returns the previous marker (nested actions restore it). The
// marker rides genState.ctrCellHit's high bits, so the batch costs no State
// bytes. Callers are the counter actions gorge brackets (effPutCounter /
// effPutCounterAll): the multi-recipient PutCounter loops (PutCounterAll,
// PutCounterDivided, PutCounter Remembered and the attacker-group put) and
// the Support$ pick loop. A multiply sweep, the entry counters and a
// single-recipient put need no marker: gorge queues one watcher per recipient
// event there.
func (s *State) ctrAllBatchOpen() uint8 {
	old := s.g.ctrCellHit >> 1
	if old == 0 {
		s.g.ctrCellHit = s.g.ctrCellHit&ctrCellHitBit | uint8(s.npend+1)<<1
	}
	return old
}

// ctrAllBatchClose restores the marker ctrAllBatchOpen returned.
func (s *State) ctrAllBatchClose(old uint8) {
	s.g.ctrCellHit = s.g.ctrCellHit&ctrCellHitBit | old<<1
}

// ctrAllJoin folds a matching event into the watcher instance already queued
// for this batch: the listener's pending trigger gains the amount and no
// second instance is queued. Only an explicitly opened action batches; every
// other put is a batch of one. The scan starts at the batch's first pending
// index, so an instance queued by an earlier action is never joined.
//
//go:noinline
func (s *State) ctrAllJoin(l trigListener, amt int16) bool {
	batch := s.g.ctrCellHit >> 1
	if DebugCtrAllPerRecipient || batch == 0 || amt <= 0 {
		return false
	}
	for i := int(batch) - 1; i < int(s.npend); i++ {
		p := &s.pend[i]
		if p.obj == l.obj && p.card == l.card && p.ab == l.ab && p.srcVer == s.objs[l.obj].ver {
			p.trigAmt += amt
			DebugCtrAllJoins++
			return true
		}
	}
	return false
}
