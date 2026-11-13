package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// clearCounters is OpClearCounters (gorge effRemoveCounter with
// CounterType$ All and CounterNum$ All: "remove all counters from it"):
// every counter of every kind the battlefield permanent Obj holds is
// removed, the +1/+1 count and each general slot, one removal per kind as
// gorge emits one CounterChange per kind present. A Class level (CtrLevel)
// is a designation, not a counter (CR 716.4), and stays. A permanent with
// nothing to remove is untouched (gorge emits nothing). The kinds removed
// to none are marked seen first, as OpRemoveCounter does (gorge lists a
// kind at 0).
func (s *State) clearCounters(k *prep, in *kop.Instr) {
	sl := s.objRef(k, in.Obj)
	if sl == NoSlot {
		return
	}
	p := &s.perms[sl]
	any := p.p1 != 0
	for i := range p.ck {
		any = any || p.cn[i] != 0 && kop.CounterKind(p.ck[i]) != kop.CtrLevel
	}
	if p.ck[ctrSlots-1] != 0 && s.ctrOverSum(p.obj, 0) != 0 {
		any = true
	}
	if !any {
		return
	}
	s.touchSl(sl)
	p.seeCtr(s)
	p.setCtrN(s, int(kop.CtrP1P1), 0)
	for again := true; again; {
		again = false
		for i := range p.ck {
			if p.cn[i] != 0 && kop.CounterKind(p.ck[i]) != kop.CtrLevel {
				// setCtrN may free the slot and shift the ones above it
				// down: rescan from the first slot
				p.setCtrN(s, int(p.ck[i]), 0)
				again = true
				break
			}
		}
		for _, e := range s.ctrOverOf(p.obj, 0) {
			if e.n != 0 && kop.CounterKind(e.kind) != kop.CtrLevel {
				p.setCtrN(s, int(e.kind), 0)
				again = true
				break
			}
		}
	}
}

// removeCounterAll is OpRemoveCounterAll (gorge effRemoveCounterAll): each
// battlefield permanent matching F, in gorge's sweep order (seat 0's
// battlefield in entry order, then seat 1's: sweepHits), loses A counters of
// kind C, or with B.N&1 (AllCounters$ True) every counter of that kind it
// has, read first so nothing is overstated; a permanent with none of the
// kind is untouched (gorge's zero-batch no-op). The kind removed to none is
// marked seen, as OpRemoveCounter does.
func (s *State) removeCounterAll(k *prep, in *kop.Instr) {
	n := s.value(k, in.A)
	if n < 0 {
		n = 0
	}
	all := in.B.N&1 != 0 // seat-ok: an instruction flag bit
	var ord [MaxPerm]Slot
	cnt := s.sweepHits(k, in.F, &ord)
	for _, sl := range ord[:cnt] {
		p := &s.perms[sl]
		amt := n
		if all {
			amt = p.ctrN(s, int(in.C))
		}
		if amt <= 0 || p.ctrN(s, int(in.C)) == 0 {
			continue
		}
		s.touchSl(sl)
		p.seeCtr(s)
		p.subCtrN(s, int(in.C), amt)
	}
}
