package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// OpMoveCounterKind and OpMoveCounterAll (gorge effMoveCounter's
// CounterType$ Any kind pick and its CounterType$ All every-kind walk,
// CR 122.5). Both move counters from the origin in Ref C onto the
// destination Obj exactly as OpMoveCounter (dirac/ctr_move.go) does one
// named kind: the origin loses them and the destination gains them once,
// through the counter replacements, firing the destination's counter-added
// triggers. The All walk takes the kinds in ascending kernel kind order;
// gorge's is its carrier's slice order, and the oracle compares the kind
// pick by kind (counterKindAction), so the orders are presentation
// details. Cold: reached only from the execCold chain.

// moveKinds writes to out the counter kinds of the permanent at sl that
// gorge's counterKinds() walks with a positive count, in ascending kind
// order: the Class level and the other non-kinds heldKinds can carry are
// excluded by IsCounterKind, and the regeneration shield is
// state.InternalCounterMarker, no counter (CR 122.1) that an every-kind
// move names. CtrOther is the kernel's no-slot filler: gorge's counterKinds
// would name such a kind, but kgen refuses a put of a kind it cannot map
// (kgen/build.go, counterKind), so no kernel code ever holds counters there
// and the skip (the ctr_removeany.go / ctr_addorremove.go convention) can
// never drop a kind gorge names.
func (s *State) moveKinds(sl Slot, out *ctrHeld) int {
	n := 0
	var held ctrHeld
	p := &s.perms[sl]
	for _, c := range held[:p.heldKinds(s, &held)] {
		if !kop.IsCounterKind(int(c)) || c == uint16(kop.CtrOther) || c == uint16(kop.CtrShield) {
			continue
		}
		if p.ctrN(s, int(c)) > 0 {
			out[n] = c
			n++
		}
	}
	return n
}

// moveOneKind moves up to want counters of kind (want < 0: all of them)
// from src to dst: the moveCounter walk of ctr_move.go for one kind.
func (s *State) moveOneKind(k *prep, src, dst Slot, kind, want int) {
	ps := &s.perms[src]
	n := ps.ctrN(s, kind)
	if want >= 0 && n > want {
		n = want
	}
	if n <= 0 {
		return
	}
	ps.seeCtr(s)
	s.subCtrFire(k, src, kind, n) // trig_counterremoved.go
	s.touchSl(src)
	n = s.placeCounters(k, dst, kind, n, s.perms[dst].ctrl, ctrPutEffect) // as OpPutCounter

	pd := &s.perms[dst]
	if n > 0 && k.anyCtrAdded {
		s.ctrNote(k, dst, kind)
	}
	if n > 0 && s.evLive(kop.TrigCounterAddedOnce) {
		s.g.lastCK = kop.CounterKind(kind)
		s.fireObj(k, kop.TrigCounterAddedOnce, dst, pd.obj, pd.ctrl, int16(n))
	}
}

// moveCounterAll is OpMoveCounterAll (The Ozolith's begin-combat move):
// every counter kind the origin holds leaves it with its whole count onto
// the destination, kinds in kernel kind order (gorge's AllKinds walk, one
// moveCounterMove per kind).
func (s *State) moveCounterAll(k *prep, in *kop.Instr) {
	src := s.objRef(k, kop.Ref(in.C))
	dst := s.objRef(k, in.Obj)
	if src == NoSlot || dst == NoSlot || src == dst {
		return
	}
	var held ctrHeld
	for _, c := range held[:s.moveKinds(src, &held)] {
		s.moveOneKind(k, src, dst, int(c), -1)
	}
}

// moveCounterKind starts OpMoveCounterKind (Nesting Grounds' "move a
// counter"): with more than one kind on the origin the ability's controller
// picks which moves (gorge's "move_counter_kind" KChoose, Min 1 / Max 1); a lone
// kind is taken unasked and no counter means no ask (the strict-supersets
// discipline: a Max-0 or Max-1 election is no election). false = suspended.
func (s *State) moveCounterKind(k *prep, in *kop.Instr) bool {
	src := s.objRef(k, kop.Ref(in.C))
	dst := s.objRef(k, in.Obj)
	if src == NoSlot || dst == NoSlot || src == dst {
		return true
	}
	var held ctrHeld
	n := s.moveKinds(src, &held)
	if n == 0 {
		return true
	}
	if n == 1 {
		s.moveOneKind(k, src, dst, int(held[0]), s.value(k, in.A))
		return true
	}
	// The ability's controller picks, gorge's c.Controller chooser
	// (engine/effects/counters.go, the "move_counter_kind" KChoose): the
	// origin may be an opponent's permanent (Rikku, Resourceful Guardian's
	// Creature.OppCtrl origin), as ctr_putexisting.go poses its kind pick.
	s.res.ask, s.res.askP = raMoveCounterKind, s.res.e.ctrl
	return false
}

// moveCounterKindOptions poses gorge's kind pick: one option per kind the
// origin still holds, in kernel kind order, the answer valued by the kind.
//
//kmanifest:askopts raMoveCounterKind
func (s *State) moveCounterKindOptions(k *prep, d *Decision) {
	in := &s.prog(k)[s.res.pc]
	src := s.objRef(k, kop.Ref(in.C))
	var held ctrHeld
	n := 0
	if src != NoSlot {
		n = s.moveKinds(src, &held)
	}
	if n == 0 {
		// Unreachable in play (nothing runs between the suspension and
		// this ask's options): an origin that went out of the kernel's
		// reach poses a lone Done, which the answer closes without moving.
		d.add(MakeAction(AkDone, 0, 0, 0))
		return
	}
	d.Kind, d.Sub = DkChoose, ChMoveCounterKind
	for _, c := range held[:n] {
		d.add(MakeAction(AkChoose, 0, 0, uint8(c)))
	}
}

// moveCounterKindAnswer takes the kind pick: A counters of that kind move
// and the program continues.
//
//kmanifest:askapply raMoveCounterKind
func (s *State) moveCounterKindAnswer(k *prep, a Action) {
	in := &s.prog(k)[s.res.pc]
	src := s.objRef(k, kop.Ref(in.C))
	dst := s.objRef(k, in.Obj)
	if a.Kind() != AkDone && src != NoSlot && dst != NoSlot && src != dst {
		s.moveOneKind(k, src, dst, int(a.Mode()), s.value(k, in.A))
	}
	s.continueRes(k, 0)
}
