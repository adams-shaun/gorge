package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// lkiCountersAll is the sum of every counter but a Class's level on LKI record
// i (lk's 1-based index), the CtrAll read of a snapshot.
func (s *State) lkiCountersAll(i uint8) int {
	l := s.lk(i)
	n := int(l.p1p1)
	for j := range l.ck {
		if l.ck[j] != 0 && kop.CounterKind(l.ck[j]) != kop.CtrLevel {
			n += int(l.cn[j])
		}
	}
	return n + s.ctrOverSum(l.obj, i)
}

// refCountersAll is refCounters for kop.CtrAll: every counter on the
// object summed (gorge Object.Counter("ALL") / sumCounters, Warden of the
// Inner Sky). The kernel's level slot is a designation, not a counter
// (CR 716; gorge ClassLevelValue), so it is left out; the deathtouch mark
// and the regeneration shield are no counter slots here, the markers
// gorge's InternalCounterMarker excludes.
func (s *State) refCountersAll(x *kop.Expr, r kop.Ref, ctx evalCtx) int {
	e := ctx.e
	// x.N 1 marks a TriggeredAttacker$CardCounters.ALL
	// read, which gorge evaluates LIVE even when the trigger captured the
	// attacker's LKI for another instruction (the sibling
	// TriggeredAttackerLKICopy PutCounter): the LKI substitution below is
	// for TriggeredCard$CardCounters in leaves triggers, not this head.
	live := x.N == 1
	if DebugCountHeads11Off && live {
		return 0 // TestValueHeadsTrackedObjectMutation: the attacker's live counter read
	}
	if !live && (r == kop.RefDelayedRemembered || r == kop.RefTriggerObj && !DebugTrigCtrLive) && e != nil && e.li != 0 {
		return s.lkiCountersAll(e.li)
	}
	var o Obj
	if r == kop.RefSelf {
		o = ctx.src
	} else {
		o = s.refObjCtx(r, ctx)
	}
	sl := NoSlot
	if r == kop.RefSelf {
		sl = s.selfCountersLKI(e, o) // a departed source's last counters (CR 608.2h)
	}
	if sl == NoSlot && r == kop.RefSelf {
		if n, ok := s.selfDepartCtr(e, o, kop.CtrAll); ok {
			return n // its slot was reused: the departure snapshot's (ctr_selfdepart.go)
		}
	}
	if sl == NoSlot {
		if o == NoObj || s.objs[o].zone != kop.ZoneBattlefield {
			return 0
		}
		sl = s.objs[o].slot
	}
	p := &s.perms[sl]
	n := int(p.p1)
	for i := range p.ck {
		if kop.CounterKind(p.ck[i]) != kop.CtrLevel {
			n += int(p.cn[i])
		}
	}
	if p.ck[ctrSlots-1] != 0 {
		n += s.ctrOverSum(p.obj, 0)
	}
	return n
}
