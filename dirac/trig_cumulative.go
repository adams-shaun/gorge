package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Cumulative upkeep (CR 702.24a; Mystic Remora). gorge startCumulativeUpkeep
// puts an age counter on the source (an ordinary CounterChange emit, so an
// AddCounter replacement applies) and asks the trigger's controller to pay
// the cost once per age counter or sacrifice the source: the pay-or-decline
// ask is the same resPay window as any trigger's own cost, its two options
// canonically trigger_cost_pay and trigger_cost_decline (oracle canon.go).
//
// kgen lowers the keyword to
//
//	ExtCumulative C=2        the age counter; skips the next C instructions when the source is gone
//	UnlessPaid You Mana n*AGE  the cost once per age counter, skipping the sacrifice when paid
//	Sacrifice self           a decline
//
// The UnlessPaid is posed as the trigger's own cost (ChGenPay, not an unless
// choice's ChGenPayMode) because it follows ExtCumulative (afterCumulative).

// cumulativeStart is ExtCumulative.
//
//go:noinline
func (s *State) cumulativeStart(k *prep, in *kop.Instr) {
	sl := s.objRef(k, kop.RefSelf)
	if sl == NoSlot {
		s.res.pc += uint8(in.C) // gorge: the source left the battlefield, nothing is asked
		return
	}
	kind := int(kop.CtrAge)
	if ro := &s.g.ro; k.gen.anyRepl&(rbCtrOn|rbPlusCtrEOT) != 0 && ro.n == 0 && !s.putCounterBlocked(k, sl, kind) {
		if c := s.ctrGather(k, sl, kind); c.ok && c.n == 2 && c.mixed() {
			s.capFull(StallCtrOrder) // gorge would ask the CR 616.1 order inside the emit
			return
		}
	}
	s.g.lastCK = kop.CounterKind(kind) // CounterAddedOnce reads the kind
	m := s.placeCounters(k, sl, kind, 1, s.perms[sl].ctrl, ctrPutEffect)
	if m > 0 && k.anyCtrAdded {
		s.ctrNote(k, sl, kind)
	}
	if m > 0 && s.subs[kop.TrigCounterAddedOnce].any() {
		s.fireObj(k, kop.TrigCounterAddedOnce, sl, s.perms[sl].obj, s.perms[sl].ctrl, int16(m))
	}
}

// afterCumulative reports whether the instruction before the current one is
// ExtCumulative: the current UnlessPaid is a cumulative upkeep's own cost.
func (s *State) afterCumulative(k *prep) bool {
	pc := s.res.pc
	return pc != 0 && k.gen != nil && s.prog(k)[pc-1].Ext() == kop.ExtCumulative
}
