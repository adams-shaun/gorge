package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Renown N (CR 702.112) and the renowned designation (gorge
// Object.Renowned, the events.AlterAttribute "Renowned" fold) and the
// IsRenowned filter word. gorge's cards/kw_renown.go expands the keyword into
// a combat-damage-to-a-player self trigger under the intervening if
// Card.Self+!IsRenowned (lowered from that line), whose body is DB$
// PutCounter | Renown$ N: OpExt ExtRenown here. The designation is the arena
// slot bit mechRenowned, assigned only when the table has an ExtRenown (the
// one writer); it ends when the permanent leaves the battlefield (the leave
// path's clear; CR 702.112b). Cold and out of line.

// RenownStats counts the permanents ExtRenown made renowned (the lockstep
// test's proof that its pool reaches the designation).
var RenownStats struct{ Marked atomic.Int64 }

// renownedAt reports whether the permanent at sl is renowned.
func (s *State) renownedAt(k *prep, sl Slot) bool {
	return s.mSl(k, mechRenowned, sl)
}

// renown is ExtRenown (gorge effPutCounter's RenownSet arm): a battlefield
// source that is not renowned gets B +1/+1 counters through OpPutCounter's
// path (the counter replacements and CounterAddedOnce), then the renowned
// designation. Already renowned, gone from the battlefield (objRef's
// NoSlot), or B <= 0: nothing (gorge's resolve-time skips and its n > 0
// mark gate).
func (s *State) renown(k *prep, in *kop.Instr) {
	sl := s.objRef(k, in.Obj)
	if sl == NoSlot || s.renownedAt(k, sl) {
		return
	}
	amt := s.value(k, in.B)
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
	s.touchSl(sl)
	RenownStats.Marked.Add(1)
}
