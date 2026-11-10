package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// damage triggers that read the damaged referent. Cold and
// out of line: every hot site tests genPrep.anyDmgRef inline first.
//
// A DamageDone line carries the referent in three shapes: Filter2 (the
// damaged permanent must match; a player never does), Tf2ToYou (the damaged
// player is the listener's controller) and Tf2PerReferent (gorge Mode$
// DamageDoneOnce: queued once per damaged referent per damage batch, at the
// referent's first matching event, with the batch total of the matching
// events to that referent; trigger_match.go damageBatchKey).
//
// Only a combat damage step and ExtEachDamage (dealHitsToOne) coalesce: every
// other noncombat batch hits each referent at most once (DamageAll, a damage
// map, a fight's two hits, a replacement-mediated sequence deal one event per
// distinct recipient), so there a line is its own batch of one and queues on
// the event's amount. The step's latch is the stack-local refLatch (no State);
// the batch total is a forward scan of the step's events, so the instance is
// queued at the first event, in gorge's order, with gorge's closed amount.

// maxRefLatch is the (listener, referent) latches one combat damage step holds.
const maxRefLatch = 16

// dealtOnceMark is the rcp of a DamageDealtOnce line's once-per-batch entry.
const dealtOnceMark = 0xfe

type refEnt struct {
	obj  Obj
	card CardID
	ab   uint8
	rcp  uint8
	at   uint8 // the combat event (index + 1) that created it
}

// refLatch is a damage frame's per-referent state: a combat step (r set:
// the events to scan, the latch) or damageBatch's per-recipient pass (only:
// just the referent lines fire).
type refLatch struct {
	r    *dmgRec
	only bool
	agg  bool  // damageBatch's aggregate fire: only the once-per-batch lines queue
	at   uint8 // the combat event (index + 1) being fired
	n    int
	e    [maxRefLatch]refEnt
	// damageBatch's recipients and their damage (only)
	codes []uint8
	dn    []int16
}

// DebugDdoNoLatch queues a per-referent line at every matching event.
var DebugDdoNoLatch bool

// DebugRefLatchN, when set by a test, is told the refLatch size after each
// insert (production never writes it).
var DebugRefLatchN func(n int)

// DebugDdoNoGate ignores the DamageAmount$ gate.
var DebugDdoNoGate bool

// DebugDmgIgnoreFilter2 ignores the recipient filter of a damage trigger
// (the behaviour before the fix).
var DebugDmgIgnoreFilter2 bool

// recipientPlayer is the player a recipient code names.
func recipientPlayer(rcp uint8) (uint8, bool) {
	if isPlayerTarget(rcp) {
		return CodeSeat(rcp), true
	}
	return 0, false
}

// ddoLine checks the referent half of damage trigger line a of listener l
// for one damage event (source obj srcObj controlled by srcCtrl, slot src
// when it is a permanent) of amt to recipient rcp (NoTarget: a multi-
// recipient aggregate), and returns the queued amount, trigger object and
// trigger player. A line without a referent read queues on the event's amount.
//
//go:noinline
func (s *State) ddoLine(k *prep, l trigListener, a *kop.Ability, src Slot, srcObj Obj, srcCtrl uint8, amt int16, rcp uint8, rl *refLatch) (int16, Obj, uint8, bool) {
	t := &a.Trig
	if t.Filter2 == 0 && t.Flags2&(kop.Tf2PerReferent|kop.Tf2ToYou) == 0 {
		return amt, srcObj, 1 - srcCtrl, true
	}
	if rcp == NoTarget {
		return 0, NoObj, 0, false // damageBatch's aggregate fire: its per-recipient pass handles it
	}
	tobj, tpl, ok := s.ddoRecipient(k, l, t, srcObj, rcp)
	if !ok {
		return 0, NoObj, 0, false
	}
	if t.Flags2&kop.Tf2PerReferent == 0 {
		if t.Event == kop.TrigDamageDealtOnce && rl != nil && rl.only {
			// gorge fires a DamageDealtOnce line once per source per batch
			// with the total: at the first matching recipient, the sum of
			// every matching one.
			for i := 0; i < rl.n; i++ {
				if e := &rl.e[i]; e.obj == l.obj && e.card == l.card && e.ab == l.ab && e.rcp == dealtOnceMark {
					return 0, NoObj, 0, false
				}
			}
			if rl.n == len(rl.e) {
				s.capFull(StallRefLatch)
				return 0, NoObj, 0, false
			}
			rl.e[rl.n] = refEnt{l.obj, l.card, l.ab, dealtOnceMark, 0}
			rl.n++
			total := 0
			for j, c := range rl.codes {
				if rl.dn[j] > 0 {
					if _, _, ok := s.ddoRecipient(k, l, t, srcObj, c); ok {
						total += int(rl.dn[j])
					}
				}
			}
			return int16(total), srcObj, 1 - srcCtrl, true
		}
		return amt, srcObj, 1 - srcCtrl, true
	}
	total := int(amt)
	if rl != nil && rl.r != nil {
		for i := 0; i < rl.n; i++ {
			// an entry of an earlier event: the referent's first event queued
			// (or failed the gate) already. The copies of one granted line
			// (Strength of Will cast twice) share a key and queue together
			// at the one event, so an entry of this event does not latch.
			if e := &rl.e[i]; e.obj == l.obj && e.card == l.card && e.ab == l.ab && e.rcp == rcp && e.at != rl.at && !DebugDdoNoLatch {
				return 0, NoObj, 0, false
			}
		}
		if rl.n == len(rl.e) {
			s.capFull(StallRefLatch)
			return 0, NoObj, 0, false
		}
		rl.e[rl.n] = refEnt{l.obj, l.card, l.ab, rcp, rl.at}
		rl.n++
		if DebugRefLatchN != nil {
			DebugRefLatchN(rl.n)
		}
		if !DebugDdoNoLatch {
			total = 0
			for i := 0; i < rl.r.nev; i++ {
				ev := &rl.r.ev[i]
				if ev.n <= 0 || !s.sameReferent(rl.r, ev, rcp) || !s.objEventMatch(k, l, a, ev.src) {
					continue
				}
				total += int(ev.n)
			}
		}
	}
	if !DebugDdoNoGate && !kop.DmgGatePass(t.N, total) {
		return 0, NoObj, 0, false
	}
	return int16(total), tobj, tpl, true
}

// ddoRecipient applies the recipient half of line t (Filter2, Tf2ToYou) of
// listener l to recipient rcp, and returns the trigger object and player a
// per-referent queue carries.
func (s *State) ddoRecipient(k *prep, l trigListener, t *kop.Trigger, srcObj Obj, rcp uint8) (Obj, uint8, bool) {
	if pl, ok := recipientPlayer(rcp); ok {
		if t.Filter2 != 0 && t.Flags&kop.TfToPlayer == 0 && !DebugDmgIgnoreFilter2 || t.Flags2&kop.Tf2ToYou != 0 && pl != l.ctrl {
			return NoObj, 0, false
		}
		return srcObj, pl, true
	}
	ob := &s.objs[rcp]
	if ob.zone != kop.ZoneBattlefield {
		return NoObj, 0, false
	}
	sl := ob.slot
	if t.Filter2 != 0 && !DebugDmgIgnoreFilter2 && !s.matchPermGen(k, t.Filter2, sl, l.ctrl, l.sl, layerFull) {
		return NoObj, 0, false
	}
	return Obj(rcp), s.perms[sl].ctrl, true
}

// sameReferent: combat event ev lands on the referent rcp names.
func (s *State) sameReferent(r *dmgRec, ev *dmgEv, rcp uint8) bool {
	if ev.dst == NoSlot {
		_, ok := recipientPlayer(rcp)
		return ok
	}
	return isObjTarget(rcp) && s.perms[ev.dst].obj == Obj(rcp)
}

// combatRcp is the recipient code of combat event ev (its player the
// defending player, 1 - the active player).
func (s *State) combatRcp(ev *dmgEv) uint8 {
	if ev.dst == NoSlot {
		return PlayerTarget0 + 1 - s.active
	}
	return uint8(s.perms[ev.dst].obj)
}

// fireDamageRefs is damageBatch's per-recipient pass: one Damage event per
// recipient that took damage, for every line but a DamageDealtOnce line with
// no referent read, which the aggregate fire queued.
//
//go:noinline
func (s *State) fireDamageRefs(k *prep, codes []uint8, dn []int16) {
	var rl refLatch
	rl.only = true
	rl.codes, rl.dn = codes, dn
	for i, code := range codes {
		if dn[i] <= 0 {
			continue
		}
		if sl := s.objRef(k, kop.RefSelf); sl != NoSlot {
			s.fireDamageEvGen(k, sl, dn[i], code >= PlayerTarget0, false, nil, code, &rl)
		} else if s.res.e.obj != NoObj {
			s.fireDamageSrc(k, s.res.e.obj, s.res.e.ctrl, dn[i], dmgPl(code), code, &rl)
		}
	}
}

// batchLineSkips reports a line damageBatch's other fire queues: the aggregate
// fire is for the DamageDealtOnce lines with no referent read, the per-recipient
// pass for the rest (gorge: a Damage event per recipient; DamageDealtOnce once
// per source per batch).
func (rl *refLatch) batchLineSkips(t *kop.Trigger) bool {
	if rl == nil || !rl.agg && !rl.only {
		return false
	}
	once := t.Event == kop.TrigDamageDealtOnce && t.Filter2 == 0 && t.Flags2&(kop.Tf2PerReferent|kop.Tf2ToYou) == 0
	return once == rl.only
}
