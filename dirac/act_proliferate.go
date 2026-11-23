package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// --- proliferate (CR 701.34a) ---
//
// gorge effProliferate: Amount$ proliferate actions, each a fresh any-number
// choice (Min 0) by the resolving controller over the permanents with a
// real counter (battlefield, seat 0 first, then seat 1; the regeneration
// shield and the deathtouch mark are internal markers, never counters
// here: hasRealCounters) and then the players with one (poison, energy);
// each chosen recipient gets +1 of each kind it has, and the completed
// action fires "whenever you proliferate" (events.Proliferate) even when
// nothing was eligible. The kernel keeps the picks in res.arr then res.peek and applies them, in
// answer order, when the ask closes (gorge applies the whole answer at its
// commit, effects/counters.go applyProliferate): a placement that holds a CR
// 616.1 counter-order ask (a doubler beside a plus-N, repl_ctrorder.go) parks
// there, and the cursor (res.peekI the pick, res.topN the held kind) resumes it.
// res.left counts the actions still owed. gorge emits one event per kind in
// the recipient's insertion order while the kernel walks kinds ascending:
// a recipient with two kinds a counter-added trigger hears stalls
// (StallProlif) unless ctrOrderObservable proves their order unseen
// (prolifOrderObservable), as does a seventeenth pick.

const prolifMaxPicks = 16

// prolifPicked: recipient o (an Obj, or PlayerCode(p)) is picked in the
// current action.
func (s *State) prolifPicked(o TCode) bool {
	r := &s.res
	for i := 0; i < int(r.peekN); i++ {
		if s.prolifPick(i) == o {
			return true
		}
	}
	return false
}

func (s *State) prolifPick(i int) TCode {
	if i < len(s.res.arr) {
		return TCode(s.res.arr[i])
	}
	return TCode(s.res.peek[i-len(s.res.arr)])
}

// permRealCtrs is the number of counter kinds the permanent at sl holds
// with a positive count (gorge hasRealCounters / counterKinds; the
// regeneration shields and the deathtouch mark live outside the counters).
func (s *State) permRealCtrs(sl Slot) int {
	p := &s.perms[sl]
	n := 0
	var held ctrHeld
	for _, c := range held[:p.heldKinds(s, &held)] {
		if kop.IsCounterKind(int(c)) { // CtrLevel is no counter (kw_class.go)
			n++
		}
	}
	return n
}

// prolifOptions lists the unpicked eligible recipients and Done.
func (s *State) prolifOptions(k *prep, d *Decision) {
	for p := uint8(0); p < MaxSeats; p++ {
		for m := s.ctrlM[p].and(s.occ); m.any(); m = m.rest() {
			sl := m.first()
			o := s.perms[sl].obj
			if s.permRealCtrs(sl) > 0 && !s.prolifPicked(TCode(o)) {
				d.add(MakeAction(AkChoose, TCode(o), 0, 0))
			}
		}
	}
	for p := uint8(0); p < MaxSeats; p++ {
		if (s.g.poison[p] > 0 || s.g.energy[p] > 0 || s.contracts(p) > 0 || s.plCtr(p, kop.PlCtrExperience) > 0 || s.Rad(p) > 0) && !s.pl[p].lost && !s.prolifPicked(PlayerCode(p)) {
			d.add(MakeAction(AkChoose, PlayerCode(p), 0, 0))
		}
	}
	d.add(MakeAction(AkDone, 0, 0, 0))
}

// prolifApplying marks res.restSel while the picks are being applied (the
// scry rest-phase byte is unused by a proliferate): the op re-run after a
// counter-order answer resumes the cursor instead of starting afresh.
const prolifApplying = 0xa5

// proliferate is OpProliferate; false: suspended on the pick.
func (s *State) proliferate(k *prep, in *kop.Instr) bool {
	if s.res.restSel == prolifApplying {
		return s.prolifResume(k) // the order ask was answered
	}
	n := s.value(k, in.A)
	if n <= 0 {
		return true
	}
	s.res.left = uint8(min(n, 255))
	return s.prolifNext(k)
}

// prolifNext starts the next owed proliferate action: false when it asks.
func (s *State) prolifNext(k *prep) bool {
	r := &s.res
	for r.left > 0 {
		r.peekN, r.peekI, r.topN = 0, 0, 0
		var d Decision
		s.prolifOptions(k, &d)
		if d.N > 1 {
			r.ask, r.askP = raGenProliferate, r.e.ctrl
			return false
		}
		s.prolifDone(k)
	}
	return true
}

// prolifDone completes one proliferate action: "whenever you proliferate".
func (s *State) prolifDone(k *prep) {
	s.res.left--
	s.fireProliferate(k, s.res.e.ctrl)
}

// answerProlif records one pick (or Done) of the proliferate ask; Done, or
// the last pick, applies them all.
func (s *State) answerProlif(k *prep, a Action) {
	r := &s.res
	if a.Kind() != AkDone {
		o := a.Source()
		if int(r.peekN) >= prolifMaxPicks {
			s.capFull(StallProlif)
		} else {
			if int(r.peekN) < len(r.arr) {
				r.arr[r.peekN] = Obj(o)
			} else {
				r.peek[int(r.peekN)-len(r.arr)] = Obj(o)
			}
			r.peekN++
		}
		var d Decision
		s.prolifOptions(k, &d)
		if d.N > 1 {
			return // more to pick
		}
	}
	r.ask = raNone
	r.restSel = prolifApplying
	if !s.prolifApplyAll(k) {
		s.ctl = ctlResolve // parked on the counter-order ask
		return
	}
	if s.prolifFinish(k) {
		s.continueRes(k, 0)
	}
}

// prolifResume runs the op again after a counter-order answer: false when it
// asks again or the next owed action asks.
func (s *State) prolifResume(k *prep) bool {
	if !s.prolifApplyAll(k) {
		return false
	}
	return s.prolifFinish(k)
}

// prolifFinish ends the applied action and starts the next owed one: false
// when that one asks.
func (s *State) prolifFinish(k *prep) bool {
	s.res.restSel, s.res.peekI, s.res.topN = 0, 0, 0
	s.prolifDone(k)
	return s.prolifNext(k)
}

// prolifApplyAll applies the picks from the cursor on; false: parked on a
// counter-order ask.
func (s *State) prolifApplyAll(k *prep) bool {
	r := &s.res
	for r.peekI < r.peekN {
		if !s.prolifApply(k, s.prolifPick(int(r.peekI))) {
			return false
		}
		r.peekI++
		r.topN = 0
	}
	return true
}

// prolifApply gives recipient o another counter of each kind it has; false:
// parked on the CR 616.1 order ask of one placement (cursor res.topN).
func (s *State) prolifApply(k *prep, o TCode) bool {
	if isPlayerOrNone(o) {
		p := CodeSeat(o)
		if s.g.poison[p] > 0 {
			s.addPoison(p, 1)
		}
		if s.g.energy[p] > 0 {
			s.g.energy[p] = uint8(min(int(s.g.energy[p])+1, 255))
		}
		if s.contracts(p) > 0 {
			s.addContract(k, p, 1) // ctr_contract.go
		}
		if c := s.plCtrPtr(p, kop.PlCtrExperience); c != nil && *c > 0 {
			*c = uint8(min(int(*c)+1, 255))
		}
		if s.Rad(p) > 0 {
			s.addRad(k, p, 1) // act_radiation.go
		}
		return true
	}
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield || ob.slot == NoSlot {
		return true
	}
	sl := ob.slot
	if s.res.topN == 0 && s.subs[kop.TrigCounterAddedOnce].any() && s.permRealCtrs(sl) > 1 && s.prolifOrderObservable(k, sl) {
		s.capFull(StallProlif)
	}
	// bolsterPut puts counters of kind c only, so the kinds held stay the list's
	var held ctrHeld
	kinds := held[:s.perms[sl].heldKinds(s, &held)]
	for j := int(s.res.topN); j < len(kinds); j++ {
		c := kinds[j]
		if !kop.IsCounterKind(int(c)) {
			continue // CtrLevel is no counter (kw_class.go)
		}
		if s.perms[sl].ctrN(s, int(c)) > 0 {
			if k.gen != nil && s.ctrOrderPark(k, sl, int(c), 1) {
				s.res.topN = uint8(j)
				return false // the order ask; this placement runs again once it is answered
			}
			s.bolsterPut(k, sl, int(c), 1)
		}
	}
	return true
}

// prolifOrderObservable reports whether the per-kind order of proliferating
// the permanent at sl (gorge's insertion order, the kernel's ascending kinds)
// can reach a different stack: only the kinds a counter-added trigger hears
// have a position anyone sees (ctrAddedHeard), and ctrOrderObservable decides
// for those, as for a putCountersFrom copy. A Saga's chapters hear only lore,
// so a creature with a +1/+1 and a lifelink counter beside a Saga does not stall.
func (s *State) prolifOrderObservable(k *prep, sl Slot) bool {
	putter := s.res.e.ctrl // the proliferating player puts the counters: a "whenever you put" line on the other seat hears nothing
	var heard [ctrSlots + 1 + maxCtrOver]kop.CounterKind
	n := 0
	var held ctrHeld
	for _, c := range held[:s.perms[sl].heldKinds(s, &held)] {
		if n0, _, _ := s.ctrAddedListenerBy(k, kop.CounterKind(c), putter, sl); kop.IsCounterKind(int(c)) && n0 > 0 {
			heard[n] = kop.CounterKind(c)
			n++
		}
	}
	return n > 1 && s.ctrOrderObservableBy(k, heard[:n], putter, sl)
}

// fireProliferate fires "whenever <Whose> proliferates" (TrigProliferate:
// no slot subscription, the battlefield is scanned as for a gift).
func (s *State) fireProliferate(k *prep, p uint8) {
	if !k.gen.anyProlif {
		return
	}
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.occ, &ord)
	for _, sl := range ord[:n] {
		s.slotTriggers(k, sl, func(a *kop.Ability) bool { return a.Trig.Event == kop.TrigProliferate }, func(l trigListener, a *kop.Ability) {
			s.playerTrigGen(k, kop.TrigProliferate, l, a, p, 0)
		})
	}
}
