package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// OpMoveCounterAny (gorge effMoveCounter's CounterNum$ Any over a
// ValidDefined$ destination sweep, Forgotten Ancient's "move any number of
// +1/+1 counters from CARDNAME onto other creatures"). The source's
// counters of kind B.N are offered 0..held (gorge's single-origin
// "move_counter" KChoose); an answer moves that many, distributed
// round-robin over the live destinations in battlefield order (gorge
// moveCounterMove, CR 122.5). No destination or no counter poses no ask, as
// gorge returns before its ask in those cases. Cold: reached only from
// execCold1.

// moveCounterAny starts the amount ask; false = suspended.
//
//go:noinline
func (s *State) moveCounterAny(k *prep, in *kop.Instr) bool {
	src := s.objRef(k, in.Obj)
	if src == NoSlot {
		return true
	}
	kind := int(in.B.N)
	have := s.perms[src].ctrN(s, kind)
	if have <= 0 {
		return true
	}
	var ord [MaxPerm]Slot
	cnt := s.sweepHits(k, in.F, &ord)
	found := false
	for _, sl := range ord[:cnt] {
		if sl != src {
			found = true
			break
		}
	}
	if !found {
		return true
	}
	if have > 255 {
		have = 255
	}
	s.g.pickLeft = uint8(have)
	s.g.pickP = s.perms[src].ctrl
	s.res.peekI = 0 // gorge's amount ask is a Min 0 / Max held multi-pick; the picks are in res.arr
	s.res.ask, s.res.askP = raMoveCounterAny, s.perms[src].ctrl
	return false
}

// moveCounterAnyApply is the answered amount: the move walk.
func (s *State) moveCounterAnyApply(k *prep, n int) {
	in := &s.prog(k)[s.res.pc]
	src := s.objRef(k, in.Obj)
	if src == NoSlot {
		return
	}
	kind := int(in.B.N)
	ps := &s.perms[src]
	have := ps.ctrN(s, kind)
	if n > have {
		n = have
	}
	if n <= 0 {
		return
	}
	var ord [MaxPerm]Slot
	cnt := s.sweepHits(k, in.F, &ord)
	var live [MaxPerm]Slot
	m := 0
	for _, sl := range ord[:cnt] {
		if sl != src {
			live[m] = sl
			m++
		}
	}
	if m == 0 {
		return
	}
	ps.seeCtr(s)
	s.subCtrFire(k, src, kind, n)
	s.touchSl(src)
	for j := 0; j < m; j++ {
		share := (n + m - 1 - j) / m
		if share <= 0 {
			continue
		}
		dst := live[j]
		got := s.placeCounters(k, dst, kind, share, s.perms[dst].ctrl, ctrPutEffect)
		if got > 0 && k.anyCtrAdded {
			s.ctrNote(k, dst, kind)
		}
		if got > 0 && s.subs[kop.TrigCounterAddedOnce].any() {
			pd := &s.perms[dst]
			s.g.lastCK = kop.CounterKind(kind)
			s.fireObj(k, kop.TrigCounterAddedOnce, dst, pd.obj, pd.ctrl, int16(got))
		}
	}
}

// moveCounterAnyOptions poses the amount pick: one value 0..held per pick,
// each leaving the list. Gorge builds this ask with Min 0 / Max the counters
// held, so the engine poses an any-number multi-pick whose first pick its
// effect reads as the amount; the option kind is outside ChooseKinds, so
// both sides canonicalise it as choose:other.
//
//kmanifest:askopts raMoveCounterAny
func (s *State) moveCounterAnyOptions(k *prep, d *Decision) {
	r := &s.res
	d.Kind, d.Sub = DkChoose, ChMoveCounterAny
	for n := 0; n <= int(s.g.pickLeft); n++ {
		picked := false
		for i := 0; i < int(r.peekI); i++ {
			if int(r.arr[i]) == n {
				picked = true
				break
			}
		}
		if !picked {
			d.add(MakeAction(AkChoose, 0, 0, uint8(n)))
		}
	}
	d.add(MakeAction(AkDone, 0, 0, 0))
}

// moveCounterAnyAnswer takes one pick; a closed pick moves the counters and
// continues the program, an open one re-poses the multi-pick.
//
//kmanifest:askapply raMoveCounterAny
func (s *State) moveCounterAnyAnswer(k *prep, a Action) {
	if !s.applyMoveCounterAnyPick(k, a) {
		return // re-pose: gorge's multi-pick still offers the rest
	}
	s.continueRes(k, 0)
}
