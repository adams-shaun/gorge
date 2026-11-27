package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// RefTriggerSpellTargets is the set form of RefTriggerSpellTarget: every
// distinct object target of the spell whose cast raised this SpellCast
// trigger (gorge Defined$ TriggeredSpellAbilityTargets without
// IsSingleTarget$ True). The kernel runs the op once per target, in target
// order, the way execTriggerAttackers runs an attacker group.
//
//go:noinline
func (s *State) execTriggerSpellTargets(k *prep, in *kop.Instr) {
	var objs [kop.MaxTargets + maxXTgt]Obj
	n := s.triggerSpellTargetList(s.res.e.trigObj, &objs)
	if n == 0 {
		return
	}
	switch in.Op {
	case kop.OpPumpUntilEOT:
		a, b := s.value(k, in.A), s.value(k, in.B)
		for _, o := range objs[:n] {
			if s.objs[o].zone != kop.ZoneBattlefield {
				continue
			}
			sl := s.objs[o].slot
			s.touchSl(sl)
			if !s.eotAdd(k, sl, a, b) {
				return
			}
		}
	case kop.OpGrantKeyword:
		m := kop.KwMask(uint16(in.C)) | kop.KwMask(uint16(in.B.N))<<16
		for _, o := range objs[:n] {
			if s.objs[o].zone != kop.ZoneBattlefield {
				continue
			}
			sl := s.objs[o].slot
			s.grantEOT(sl, m)
			s.promoKw(k, sl, m)
			s.touchSl(sl)
		}
	case kop.OpGainControl:
		var list [MaxPerm]Slot
		cnt := 0
		for _, o := range objs[:n] {
			if s.objs[o].zone != kop.ZoneBattlefield {
				continue
			}
			list[cnt] = s.objs[o].slot
			cnt++
		}
		s.gainControlSlots(k, in, list[:cnt])
	default:
		panic("kernel: RefTriggerSpellTargets under an op that does not run it")
	}
}
