package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Cold arms: out of line,
// reached only from cold branches.

// spareRegenX holds a permanent's regeneration shields beyond the first
// (pfRegen), in bits 2-3 of perm.spare: gorge's effRegenerate adds one
// "Shield" marker counter per resolution and ReplaceDestruction spends one per
// destruction (CR 701.19a), so shields stack. A third bit is in the arena's
// mechRegen mask (kcaps2x doubled the cap 4 -> 8 for 8 B of State): eight
// shields at most; a ninth stalls (StallRegen) rather than being dropped.
// perm.flags holds no free bit for them (pfBatchGone, pfLastBear and
// pfSkipUntap own 13-15). The bits are meaningful only while pfRegen is set:
// the first shield clears them, and the cleanup step clears the whole count.
const (
	spareRegenShift       = 2
	spareRegenX     uint8 = 3 << spareRegenShift
)

// regenShields is gorge's "Shield" marker count on p without the arena's bit.
func (p *perm) regenShields() int {
	if p.flags&pfRegen == 0 {
		return 0
	}
	if DebugRegenShieldsInFlags {
		return 1 + int(p.flags>>13&3)
	}
	return 1 + int(p.spare&spareRegenX>>spareRegenShift)
}

// regenShieldsAt is regenShields with the arena mechRegen mask's third bit.
func (s *State) regenShieldsAt(sl Slot) int {
	n := s.perms[sl].regenShields()
	if n != 0 && s.mSl(s.k(), mechRegen, sl) {
		n += 4
	}
	return n
}

// setRegenShields stores n >= 1 shields on slot sl.
func (s *State) setRegenShields(sl Slot, n int) {
	p := &s.perms[sl]
	x := n - 1
	p.flags |= pfRegen
	if DebugRegenShieldsInFlags {
		p.flags = p.flags&^(3<<13) | uint16(x&3)<<13 // the old layout, over pfBatchGone and pfLastBear
	} else {
		p.spare = p.spare&^spareRegenX | uint8(x&3)<<spareRegenShift
	}
	if x&4 != 0 {
		s.mSlSet(s.k(), mechRegen, sl)
	} else {
		s.mSlClr(s.k(), mechRegen, sl)
	}
}

// addRegenShield is OpRegenerate: one more this-turn shield (gorge
// effRegenerate, CR 701.19a).
//
//go:noinline
func (s *State) addRegenShield(sl Slot) {
	s.touchSl(sl)
	switch n := s.regenShieldsAt(sl); {
	case n < 8:
		s.setRegenShields(sl, n+1)
	default:
		s.capFull(StallRegen) // a ninth shield: spare's two bits and mechRegen's hold eight (kverify-c4: Fog of Gnats' fifth; kcaps2x 4 -> 8)
	}
}

// spendRegenShield removes one shield (gorge ReplaceDestruction's
// CounterChange Shield -1).
func (s *State) spendRegenShield(sl Slot) {
	switch n := s.regenShieldsAt(sl); {
	case n <= 1:
		s.perms[sl].flags &^= pfRegen
		s.perms[sl].spare &^= spareRegenX
		s.mSlClr(s.k(), mechRegen, sl)
	default:
		s.setRegenShields(sl, n-1)
	}
}

// FeatRegenShields is gorge's "Shield" marker counter on object o (the
// regeneration shields it has this turn). A phased-out permanent still holds
// them: phasing is not a zone change (CR 702.26d) and the shield is a
// this-turn effect (CR 701.19c), so a phasing table reads the slot on the
// phased zones too (kw_phasing.go); phaseOut keeps pfRegen, spare and the
// mechRegen mask.
func (s *State) FeatRegenShields(o Obj) uint8 {
	if ob := &s.objs[o]; ob.zone == kop.ZoneBattlefield && ob.slot != NoSlot ||
		!DebugPhasedShieldsZero && s.k().phasing && ob.zone.Phased() && ob.slot != NoSlot {
		return uint8(s.regenShieldsAt(ob.slot))
	}
	return 0
}

// anyRegen reports whether a permanent of hit has a regeneration shield.
func (s *State) anyRegen(hit slotMask) bool {
	for m := hit; m.any(); m = m.rest() {
		if s.canRegen(m.first()) {
			return true
		}
	}
	return false
}

// regenAll is OpDestroyAll's regeneration (gorge effDestroyAll ->
// ReplaceDestruction per victim): each shielded victim regenerates instead
// and leaves hit.
//
//go:noinline
func (s *State) regenAll(hit slotMask) slotMask {
	var out slotMask
	for m := hit; m.any(); m = m.rest() {
		sl := m.first()
		if s.canRegen(sl) {
			s.regenerate(sl)
			continue
		}
		out = out.with(sl)
	}
	return out
}

// Arms of greedy classes 154-355, the kgen-side classes.
// Cold code, reached through one cheap operand
// test at each hot site.

// destroyNoRegen is OpDestroy with NoRegen$ True (gorge effDestroy: the
// regeneration replacement is skipped when NoRegen$ is "True"; an
// indestructible permanent still survives, CR 701.19c).
//
//go:noinline
func (s *State) destroyNoRegen(k *prep, sl Slot) {
	if s.destroySpared(k, sl, true) {
		return
	}
	s.leaveParked(k, sl)
}

// destroyNoRegenNow is destroyNoRegen for a caller that reads the object right
// after the move (destroyRemember's RememberDestroyed$): a competing move is
// not parked, so the read sees the move. It stalls by name as before.
func (s *State) destroyNoRegenNow(k *prep, sl Slot) {
	if s.destroySpared(k, sl, true) {
		return
	}
	s.leave(k, sl, kop.ZoneGraveyard, false)
}

// DebugRegenShieldsInFlags (TestPermFlagsRegenBitCollisionMutation): the
// shields beyond the first live in perm.flags bits 13-14 again, where
// pfBatchGone and pfLastBear also live.
var DebugRegenShieldsInFlags bool

// DebugPhasedShieldsZero (TestPhasedOutRegenShieldsProjectionMutation):
// FeatRegenShields and FeatDeathtouchMark read zero on a phased-out permanent
// again, the pre-fix behaviour. Phasing is not a zone change (CR 702.26d), so
// the shields and the mark persist through it.
var DebugPhasedShieldsZero bool
