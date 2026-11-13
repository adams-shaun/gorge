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
// nothing was eligible. The kernel applies each pick as it is answered
// (gorge applies the whole answer at its commit: the oracle compares at
// gorge's commit boundaries) and keeps the picks in res.arr then res.peek.
// res.left counts the actions still owed. gorge emits one event per kind in
// the recipient's insertion order while the kernel walks kinds ascending:
// with a counter-added trigger in the table a two-kind recipient stalls
// (StallProlif), as does a seventeenth pick.

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
	for c := 0; c < int(kop.NumCounterKinds3); c++ {
		if !kop.IsCounterKind(c) {
			continue // CtrLevel is no counter (kw_class.go)
		}
		if p.ctrN(s, c) > 0 {
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
		if (s.g.poison[p] > 0 || s.g.energy[p] > 0) && !s.pl[p].lost && !s.prolifPicked(PlayerCode(p)) {
			d.add(MakeAction(AkChoose, PlayerCode(p), 0, 0))
		}
	}
	d.add(MakeAction(AkDone, 0, 0, 0))
}

// proliferate is OpProliferate; false: suspended on the pick.
func (s *State) proliferate(k *prep, in *kop.Instr) bool {
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
		r.peekN = 0
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

// answerProlif applies one pick (or Done) of the proliferate ask.
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
		s.prolifApply(k, TCode(o))
		var d Decision
		s.prolifOptions(k, &d)
		if d.N > 1 {
			return // more to pick
		}
	}
	r.ask = raNone
	s.prolifDone(k)
	if s.prolifNext(k) {
		s.continueRes(k, 0)
	}
}

// prolifApply gives recipient o another counter of each kind it has.
func (s *State) prolifApply(k *prep, o TCode) {
	if isPlayerOrNone(o) {
		p := CodeSeat(o)
		if s.g.poison[p] > 0 {
			s.addPoison(p, 1)
		}
		if s.g.energy[p] > 0 {
			s.g.energy[p] = uint8(min(int(s.g.energy[p])+1, 255))
		}
		return
	}
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield || ob.slot == NoSlot {
		return
	}
	sl := ob.slot
	if s.subs[kop.TrigCounterAddedOnce].any() && s.permRealCtrs(sl) > 1 {
		s.capFull(StallProlif)
	}
	for c := 0; c < int(kop.NumCounterKinds3); c++ {
		if !kop.IsCounterKind(c) {
			continue // CtrLevel is no counter (kw_class.go)
		}
		if s.perms[sl].ctrN(s, c) > 0 {
			s.bolsterPut(k, sl, c, 1)
		}
	}
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
