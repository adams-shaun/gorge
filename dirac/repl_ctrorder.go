package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The CR 616.1 counter-order ask. A counter doubler and a counter plus-N that
// both match one placement do not commute (doubling then adding one is not
// adding one then doubling), so gorge poses the competition on the
// CounterChange event (replacement_counter.go poseAddCounterOrderChoice): the
// controller of the recipient picks which applies first. The candidates are,
// in gorge's scan order, Brad Boimler's until-end-of-turn effects first (one
// per activation, the activating Boimler as the source) and then the
// filtered replacements of each player's battlefield in entry order.
//
// Only the two-candidate competition (one doubler, one plus-N) is modelled:
// after the pick one candidate is left, so no re-ask follows. With three or
// more gorge asks again over ALL the candidates, the applied one included
// (askReplacementChoice lists rc.cands, not what is left), which the kernel
// does not reproduce; those still stall (StallReplCtr).
//
// The kernel poses the same ask for an OpPutCounter of a resolving program,
// before the placement: the resolution pauses on raGenCtrOrder, the answers
// pick is kept in the order header (s.g.ro), and the same instruction runs
// again with the order settled (ctrOrderApply). Every other route, a doubler
// that is not a filtered battlefield replacement (Doubling Season, Innkeeper's
// Talent), a minus-N, three or more candidates and Boimler activations from
// two sources stall (StallReplCtr).

// DebugCtrOrderIgnore is the mutation switch: the ask is posed but the
// answers are ignored (the candidates apply in scan order), so a game whose
// controller picks the other order diverges from gorge.
var DebugCtrOrderIgnore bool

// CtrOrderStats counts the lockstep test's coverage: asks
// posed.
var CtrOrderStats struct{ Asked atomic.Int64 }

// raGenCtrOrder is the resolution ask of the counter-order competition.
const raGenCtrOrder = raGenManifestDread + 76

// ctrOrdTag marks the order header (replHdr.ac) as the counter-order ask's;
// its low bits count the picks made (one). replHdr.an holds the pick; prev
// the doublers (a bit per candidate).
const ctrOrdTag = 0xc0

// ctrCands are the counter replacements matching one placement, in gorge's
// scan order.
type ctrCands struct {
	n   int
	src [4]Obj
	amt [4]int16
	dbl uint8 // bit per candidate: a doubler
	ok  bool  // false: a shape this model does not order
}

// add appends a candidate.
func (c *ctrCands) add(src Obj, dbl bool, amt int16) {
	if c.n == len(c.src) {
		c.ok = false
		return
	}
	c.src[c.n], c.amt[c.n] = src, amt
	if dbl {
		c.dbl |= 1 << c.n
	}
	c.n++
}

// mixed: the candidates hold a doubler and a plus-N, so their order matters.
func (c *ctrCands) mixed() bool {
	return c.n > 1 && c.dbl != 0 && c.dbl != uint8(1<<c.n)-1
}

// ctrGather collects the replacements matching a placement of kind counters
// on sl: the same matches replCtrOnX counts.
func (s *State) ctrGather(k *prep, sl Slot, kind int) (c ctrCands) {
	g := k.gen
	ctrl := s.perms[sl].ctrl
	c.ok = true
	if g.anyRepl&rbDblCtr != 0 && s.replN(k, ctrl, rbDblCtr) > 0 ||
		g.anyRepl&rbDblCtrYou != 0 && s.dblCtrYou(k, s.counterAdder()) > 0 {
		c.ok = false // an EffectOnly$ or putter doubler keeps the stall
	}
	if !DebugCtrPlusEOTOff && g.anyRepl&rbPlusCtrEOT != 0 {
		for i := uint8(0); i < s.g.plusCtr[ctrl] && c.ok; i++ {
			if s.g.plusSrc[ctrl] == NoObj {
				c.ok = false // activations of two Boimlers
			}
			c.add(s.g.plusSrc[ctrl], false, 1)
		}
		if g.anyPlusCre {
			cn, src := s.plusCreN(k, sl, kind) // Prairie Dog's (repl_turnrepl.go)
			for i := uint8(0); i < cn && c.ok; i++ {
				if src == NoObj {
					c.ok = false // activations of two Prairie Dogs
				}
				c.add(src, false, 1)
			}
		}
	}
	if g.anyRepl&rbCtrOn == 0 {
		return c
	}
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		var bf [MaxPerm]Slot
		for _, b := range bf[:s.battlefieldOrder(p, s.statM.and(s.ctrlM[p]), bf[:], 0)] {
			if g.replBits[s.cardAt(b)]&rbCtrOn == 0 || s.genLost(k, b) {
				continue
			}
			lc := s.perms[b].ctrl
			cd := &k.t.Cards[s.cardAt(b)]
			for i := range cd.Abilities {
				a := &cd.Abilities[i]
				if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplDoubleCountersOn && a.Repl.Kind != kop.ReplAddCountersOn ||
					s.entryCtrSelfExcluded(k, b, sl, a.Repl.Filter) ||
					!s.ctrKindMatch(a, kind, lc) || !s.matchPerm(k, a.Repl.Filter, sl, lc, b) {
					continue
				}
				if a.Repl.Kind == kop.ReplAddCountersOn && a.Repl.A <= 0 {
					c.ok = false // a minus-N or a zero keeps the stall
				}
				c.add(s.perms[b].obj, a.Repl.Kind == kop.ReplDoubleCountersOn, int16(a.Repl.A))
			}
		}
	}
	return c
}

// ctrOrderPark poses the order ask of an OpPutCounter placement of n
// counters of kind on sl, before it happens; true when the resolution is
// suspended on it.
func (s *State) ctrOrderPark(k *prep, sl Slot, kind, n int) bool {
	g := k.gen
	ro := &s.g.ro
	if g == nil || n <= 0 || g.anyRepl&(rbCtrOn|rbPlusCtrEOT) == 0 || !s.res.active || ro.n != 0 || s.g.nc.n != 0 ||
		s.putCounterBlocked(k, sl, kind) {
		return false
	}
	c := s.ctrGather(k, sl, kind)
	if !c.ok || c.n != 2 || !c.mixed() {
		return false
	}
	ro.replHdr = replHdr{}
	ro.n, ro.cand, ro.prev, ro.p, ro.ac = uint8(c.n), c.src, c.dbl, s.perms[sl].ctrl, ctrOrdTag
	s.res.ask, s.res.askP = raGenCtrOrder, ro.p
	CtrOrderStats.Asked.Add(1)
	return true
}

// execPutCounter runs an OpPutCounter instruction; false when it parked on the
// CR 616.1 order ask (the instruction runs again once it is answered).
func (s *State) execPutCounter(k *prep, in *kop.Instr) bool {
	if in.W != kop.RefNone {
		return s.execPutCounterBy(k, in) // the Placer$ adder (act_chooseplayerres.go)
	}
	return s.execPutCounterAt(k, in)
}

// execPutCounterAt is execPutCounter's placement under the adder already
// published.
func (s *State) execPutCounterAt(k *prep, in *kop.Instr) bool {
	if sl := s.objRef(k, in.Obj); sl != NoSlot {
		amt := s.value(k, in.A)
		if k.gen != nil && s.ctrOrderPark(k, sl, int(in.C), amt) {
			return false // the CR 616.1 order ask; the instruction runs again (repl_ctrorder.go)
		}
		n := s.placeCounters(k, sl, int(in.C), amt, s.perms[sl].ctrl, ctrPutEffect)
		if n > 0 && k.anyCtrAdded {
			s.ctrNote(k, sl, int(in.C))
		}
		if in.B.N >= kop.PutCtrCellFirst {
			if in.B.N == kop.PutCtrCellFirst {
				s.g.ctrCell.clr(sl)
			}
			s.fireCounterCell(k, sl, n) // trig_countercell.go
		} else if n > 0 && s.subs[kop.TrigCounterAddedOnce].any() {
			s.fireObj(k, kop.TrigCounterAddedOnce, sl, s.perms[sl].obj, s.perms[sl].ctrl, int16(n))
		}
		if in.B.N != 0 {
			if in.B.N == 1 {
				s.putCounterRemAmount(k, in, n) // RememberAmount$ (act_remember.go)
			} else if in.B.N == 2 {
				s.putCounterRemPut(k, sl, n) // RememberPut$ (ctr_putparams.go)
			}
		}
	}
	return true
}

// applyCtrOrder answers the ask: the pick applies first, the other candidate
// after it; then the same instruction runs again.
func (s *State) applyCtrOrder(k *prep, a Action) {
	ro := &s.g.ro
	i := a.Mode()
	if i >= ro.n || ro.used&(1<<i) != 0 {
		return
	}
	ro.an, ro.ac, ro.used, ro.fin = int16(i), ctrOrdTag|1, 1<<i, true
	s.res.ask = raNone
	s.runProg(k) // the same instruction again, the order settled
	if ro.fin {
		ro.replHdr = replHdr{} // nothing consumed the answer
	}
}

// ctrOrderApply is replCtrOnX's stall site: n counters of kind on sl with an
// answered order ask applies its candidates in that order. false when no
// answer is waiting for this placement.
func (s *State) ctrOrderApply(k *prep, sl Slot, kind, n int) (int, bool) {
	ro := &s.g.ro
	if !ro.fin || ro.ac&ctrOrdTag != ctrOrdTag {
		return n, false
	}
	c := s.ctrGather(k, sl, kind)
	if !c.ok || c.n != int(ro.n) || c.dbl != ro.prev {
		return n, false
	}
	np := int(ro.ac &^ ctrOrdTag)
	if DebugCtrOrderIgnore {
		np = 0
	}
	used := uint8(0)
	step := func(i int) {
		used |= 1 << i
		if c.dbl&(1<<i) != 0 {
			n *= 2
		} else {
			n += int(c.amt[i])
		}
	}
	for j := 0; j < np; j++ {
		step(int(ro.an>>(2*j)) & 3)
	}
	for i := 0; i < c.n; i++ {
		if used&(1<<i) == 0 {
			step(i)
		}
	}
	if !DebugCtrPlusEOTOff && k.gen.anyRepl&rbPlusCtrEOT != 0 {
		CtrPlusEOTStats.Applied.Add(int64(s.g.plusCtr[s.perms[sl].ctrl]))
		if k.gen.anyPlusCre {
			cn, _ := s.plusCreN(k, sl, kind)
			TurnReplStats.Plus.Add(int64(cn))
		}
	}
	ro.replHdr = replHdr{}
	return n, true
}
