package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// ExtDistCounters (Vastwood Hydra: "you may distribute a number of +1/+1
// counters equal to the number it had among any number of creatures you
// control"). There are no targets: the resolving controller picks the
// recipients (gorge putCounterPickDistribute, a KChoose of Min 1 and Max
// min(total, eligible) over the battlefield creatures, seat 0 first) and the
// total is then divided round-robin in pick order, so the earlier picks take
// the remainder. The window and the picks ride the dig scratch (look,
// lookOrig, lookSel: digPick keeps the picks at the front in pick order).

// raGenDistPick is ExtDistCounters's pick: a recipient, or Done once one is
// picked (ChGenDistPick, source = the creature).
const raGenDistPick = raGenManifestDread + 77

// distPickStart runs ExtDistCounters: B is the total, F the recipients. It
// reports whether the program goes on (false: the pick is posed).
func (s *State) distPickStart(k *prep, in *kop.Instr) bool {
	total := s.value(k, in.B)
	if total <= 0 {
		return true
	}
	var ord [MaxPerm]Slot
	n := s.sweepHits(k, in.F, &ord)
	switch {
	case n == 0:
		return true
	case n == 1:
		// gorge asks nothing when one recipient is all there is
		s.putDividedShare(k, ord[0], kop.CtrP1P1, total)
		return true
	}
	look := &s.resArm().look
	if n > len(look) {
		s.capFull(StallLook)
		n = len(look)
	}
	for i := 0; i < n; i++ {
		look[i] = s.perms[ord[i]].obj
		s.resArm().lookOrig[i] = uint8(i)
	}
	s.g.nlook, s.g.lookSel = uint8(n), 0
	s.g.pickLeft = uint8(min(total, n))
	s.res.ask, s.res.askP = raGenDistPick, s.res.e.ctrl
	return false
}

// askOptionsDistPick builds the pick's options: each recipient not yet
// picked, then Done once one is (gorge's Min 1); false: not this ask.
func (s *State) askOptionsDistPick(k *prep, d *Decision) bool {
	if s.res.ask != raGenDistPick {
		return false
	}
	d.Sub = ChGenDistPick
	for i := 0; i < int(s.g.nlook); i++ {
		if s.g.lookSel&(1<<i) == 0 {
			d.add(MakeAction(AkChoose, uint8(s.resArm().look[i]), 0, 0))
		}
	}
	if s.g.lookSel != 0 {
		d.add(MakeAction(AkDone, 0, 0, 0))
	}
	return true
}

// applyAskDistPick applies a pick; the last one (or Done) divides the total
// among the picks. False: not this ask.
func (s *State) applyAskDistPick(k *prep, a Action) bool {
	if s.res.ask != raGenDistPick {
		return false
	}
	if a.Kind() == AkChoose {
		s.digPick(Obj(a.Source()))
		s.g.pickLeft--
		if s.g.pickLeft > 0 && bits.OnesCount32(s.g.lookSel) < int(s.g.nlook) {
			return true
		}
	}
	s.res.ask = raNone
	total := s.value(k, s.prog(k)[s.res.pc].B)
	m := bits.OnesCount32(s.g.lookSel)
	for i := 0; i < m; i++ {
		share := total / m
		if i < total%m {
			share++
		}
		if ob := &s.objs[s.resArm().look[i]]; share > 0 && ob.zone == kop.ZoneBattlefield {
			s.putDividedShare(k, ob.slot, kop.CtrP1P1, share)
		}
	}
	s.continueRes(k, 0)
	return true
}
