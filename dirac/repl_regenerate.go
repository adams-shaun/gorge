package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Cold arms: out of line,
// reached only from cold branches.

// A permanent's regeneration shields beyond the first (pfRegen) are three
// bits of the arena's slot masks: mechRegenLo (+1), mechRegenHi (+2) and
// mechRegen (+4). gorge's effRegenerate adds one "Shield" marker counter per
// resolution and ReplaceDestruction spends one per destruction (CR 701.19a),
// so shields stack: eight at most; a ninth stalls (StallRegen) rather than
// being dropped. perm.flags holds no free bit for them (pfBatchGone,
// pfLastBear and pfSkipUntap own 13-15). The masks are meaningful only while
// pfRegen is set: the first shield clears them, and the cleanup step clears
// all three.

// regenShieldsAt is gorge's "Shield" marker count on slot sl.
func (s *State) regenShieldsAt(sl Slot) int {
	p := &s.perms[sl]
	if p.flags&pfRegen == 0 {
		return 0
	}
	if DebugRegenShieldsInFlags {
		return 1 + int(p.flags>>13&3) + regenShieldsHigh(s, sl)
	}
	n := 1
	if s.mSl(s.k(), mechRegenLo, sl) {
		n++
	}
	if s.mSl(s.k(), mechRegenHi, sl) {
		n += 2
	}
	if s.mSl(s.k(), mechRegen, sl) {
		n += 4
	}
	return n
}

// regenShieldsHigh is the +4 bit of regenShieldsAt, which the mutation keeps.
func regenShieldsHigh(s *State, sl Slot) int {
	if s.mSl(s.k(), mechRegen, sl) {
		return 4
	}
	return 0
}

// setRegenShields stores n >= 1 shields on slot sl.
func (s *State) setRegenShields(sl Slot, n int) {
	p := &s.perms[sl]
	x := n - 1
	p.flags |= pfRegen
	if DebugRegenShieldsInFlags {
		p.flags = p.flags&^(3<<13) | uint16(x&3)<<13 // the old layout, over pfBatchGone and pfLastBear
	} else {
		s.mSlPut(mechRegenLo, sl, x&1 != 0)
		s.mSlPut(mechRegenHi, sl, x&2 != 0)
	}
	s.mSlPut(mechRegen, sl, x&4 != 0)
}

// mSlPut sets or clears sl in mechanic m's mask.
func (s *State) mSlPut(m mech, sl Slot, on bool) {
	if on {
		s.mSlSet(s.k(), m, sl)
	} else {
		s.mSlClr(s.k(), m, sl)
	}
}

// clearRegenShields drops every shield on slot sl.
func (s *State) clearRegenShields(sl Slot) {
	s.perms[sl].flags &^= pfRegen
	for _, m := range [...]mech{mechRegenLo, mechRegenHi, mechRegen} {
		s.mSlClr(s.k(), m, sl)
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
		s.capFull(StallRegen) // a ninth shield: the three masks hold eight (kverify-c4: Fog of Gnats' fifth; kcaps2x 4 -> 8)
	}
}

// spendRegenShield removes one shield (gorge ReplaceDestruction's
// CounterChange Shield -1).
func (s *State) spendRegenShield(sl Slot) {
	switch n := s.regenShieldsAt(sl); {
	case n <= 1:
		s.clearRegenShields(sl)
	default:
		s.setRegenShields(sl, n-1)
	}
}

// FeatRegenShields is gorge's "Shield" marker counter on object o (the
// regeneration shields it has this turn).
func (s *State) FeatRegenShields(o Obj) uint8 {
	if ob := &s.objs[o]; ob.zone == kop.ZoneBattlefield && ob.slot != NoSlot {
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
