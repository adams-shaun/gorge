package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// "distribute N +1/+1 counters among one to N targets"
// with a literal N. The slot carries TargetSpec.Divide = N; the division is
// asked after the targets (startCastDivide, CR 601.2d) and divCounters places
// it. Only the Dies-Self LKI form (Divide == DivLKIP1P1) reads the total from
// the source's last-known counters.

// DebugDivNOneEach is the mutation switch: divTotal returns the target count
// (one counter each) instead of N.
var DebugDivNOneEach bool

// divTotal is the total divCounters hands out for the slot of entry e.
func (s *State) divTotal(k *prep, e *stackEntry, slot uint8) int {
	t := tspecE(k, e, int(slot))
	if t == nil || t.Divide == kop.DivLKIP1P1 {
		return s.divLKITotal(e)
	}
	if t.Divide == kop.DivMaxV {
		return s.divMaxVTotal(k, t, e) // ctr_divided.go
	}
	if DebugDivNOneEach {
		var codes [maxXTgt + 1]uint8
		return s.divTargets(k, e, slot, &codes)
	}
	return int(t.Divide)
}

// a TargetSpec.Divide == kop.DivMaxV slot ("distribute X
// +1/+1 counters among any number of target creatures", X the spell's paid
// {X}). The slot is TgfBoundsV 0..X, so the divided total is the slot's
// evaluated maximum (gorge: TargetMax$ and DividedAsYouChoose$ are the one
// announced X). A total above what the division arrays hold stalls StallXTgt,
// like divLKITotal.

// DebugDivMaxVMinus1 is the mutation switch: a DivMaxV total is X-1.
var DebugDivMaxVMinus1 bool

// divMaxVTotal is the total for DivMaxV slot t of entry e.
func (s *State) divMaxVTotal(k *prep, t *kop.TargetSpec, e *stackEntry) int {
	_, hi := s.tgtBoundsEval(k, t, e)
	hi = s.divMaxVCap(hi)
	if DebugDivMaxVMinus1 && hi > 0 {
		hi--
	}
	return hi
}

// divMaxVCap caps an evaluated maximum at the division arrays' size.
func (s *State) divMaxVCap(hi int) int {
	if hi > maxXTgt+1 {
		s.capFull(StallXTgt)
		hi = maxXTgt + 1
	}
	return hi
}

// Ext DividedCounters (Quirion Beastcaller's "distribute X +1/+1
// counters among any number of target creatures you control, where X is the
// number of +1/+1 counters on CARDNAME"). X is the dying source's last-known
// +1/+1 counters, the trigger entry's LKI (CR 603.10a): it bounds the slot
// (0..X targets, gorge TargetMax$ X) and is the divided total. The division
// is announced after the targets (bsDivide, startCastDivide) and put as
// announced at resolution; one target takes all X unasked.

// divLKITotal is the LKI +1/+1 counters of e's triggering source, capped to
// the slot's eight objects (a ninth stalls StallXTgt).
func (s *State) divLKITotal(e *stackEntry) int {
	n := int(s.lk(e.li).p1p1)
	if n > maxXTgt+1 {
		s.capFull(StallXTgt)
		n = maxXTgt + 1
	}
	return n
}

// divLKIBounds sets slot bounds 0..X for the entry being built.
func (s *State) divLKIBounds(e *stackEntry, mt *multiTgt) {
	mt.min, mt.max = 0, uint8(s.divLKITotal(e))
}

// divCounters is ExtDividedCounters: counters per legal target in pick order.
func (s *State) divCounters(k *prep, in *kop.Instr) {
	e := &s.res.e
	slot := uint8(in.C) // C, not Obj: a RefTarget Obj would make the interpreter run the program once per target (execEachTarget)
	total := s.divTotal(k, e, slot)
	var codes [maxXTgt + 1]uint8
	var amts [maxXTgt + 1]int
	m := 0
	x := s.xtOf(e)
	if e.xi != 0 && x.div[0] != 0 {
		// the announced division: each still-legal target takes its share
		// (CR 608.2b), an illegal one's is lost
		i := 0
		if TCode(e.tgt[slot]) != NoTarget {
			if c := s.tgtCode(int(slot)); TCode(c) != NoTarget {
				codes[m], amts[m] = c, int(x.div[0])
				m++
			}
			i = 1
		}
		for j := 0; j < int(x.n); j++ {
			if x.slot[j] != slot {
				continue
			}
			if s.xtLegal(k, e, j) {
				codes[m], amts[m] = x.code[j], int(x.div[i])
				m++
			}
			i++
		}
	} else {
		m = s.divTargets(k, e, slot, &codes)
		if m == 1 {
			amts[0] = total
		} else {
			m = 0
		}
	}
	for i := 0; i < m; i++ {
		c := codes[i]
		if amts[i] <= 0 || isPlayerOrNone(TCode(c)) || s.objs[c].zone != kop.ZoneBattlefield {
			continue
		}
		s.putDividedShare(k, s.objs[c].slot, amts[i])
	}
}

// putDividedShare puts amt +1/+1 counters on the permanent at sl as one
// divided share (one CounterChange; its replacements and added-triggers).
func (s *State) putDividedShare(k *prep, sl Slot, amt int) {
	n := s.placeCounters(k, sl, int(kop.CtrP1P1), amt, s.perms[sl].ctrl, ctrPutEffect)
	p := &s.perms[sl]
	if n > 0 && k.anyCtrAdded {
		s.ctrNote(k, sl, int(kop.CtrP1P1))
	}
	if n > 0 && s.subs[kop.TrigCounterAddedOnce].any() {
		s.g.lastCK = kop.CtrP1P1
		s.fireObj(k, kop.TrigCounterAddedOnce, sl, p.obj, p.ctrl, int16(n))
	}
}
