package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Renown N (CR 702.112) and the renowned designation (gorge
// Object.Renowned, the events.AlterAttribute "Renowned" fold, the IsRenowned
// filter word). The designation is the arena slot bit mechRenowned, read
// through kop.PredPermMark (PermMarkRenowned): a card off the battlefield is
// never renowned, and a permanent taking the slot clears it (arenaClrSlot,
// CR 702.112b: it lasts only while the permanent stays on the battlefield).

// renown is OpMonstrosity with C = kop.MonstrosityRenown (gorge effPutCounter's
// RenownSet arm): a battlefield source that is not renowned gets A +1/+1
// counters through OpPutCounter's path (the counter replacements and
// CounterAddedOnce), then the renowned designation. A source that left, or is
// already renowned, does nothing; A <= 0 places no counters and never marks
// (gorge's n > 0 gate). No event fires: BecomeRenowned is not lowered.
func (s *State) renown(k *prep, in *kop.Instr) {
	sl := s.objRef(k, in.Obj)
	if sl == NoSlot || s.mSl(k, mechRenowned, sl) {
		return
	}
	amt := s.value(k, in.A)
	if amt <= 0 {
		return
	}
	s.g.lastCK = kop.CtrP1P1
	n := s.placeCounters(k, sl, int(kop.CtrP1P1), amt, s.perms[sl].ctrl, ctrPutEffect)
	p := &s.perms[sl]
	if n > 0 && k.anyCtrAdded {
		s.ctrNote(k, sl, int(kop.CtrP1P1))
	}
	if n > 0 && s.subs[kop.TrigCounterAddedOnce].any() {
		s.fireObj(k, kop.TrigCounterAddedOnce, sl, p.obj, p.ctrl, int16(n))
	}
	s.mSlSet(k, mechRenowned, sl)
	s.touchSl(sl) // the layer caches that read the designation see the change
}
