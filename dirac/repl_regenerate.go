package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Cold arms: out of line,
// reached only from cold branches.

// pfRegenX holds a permanent's regeneration shields beyond the first
// (pfRegen): gorge's effRegenerate adds one "Shield" marker counter per
// resolution and ReplaceDestruction spends one per destruction (CR
// 701.19a), so shields stack. Two bits here plus a third in
// the arena's mechRegen mask (perm.flags and cseen hold no free bit; kcaps2x doubled
// the cap 4 -> 8 for 8 B of State): eight shields at most; a ninth stalls
// (StallRegen) rather than being dropped. mechRegen's bit is meaningful only
// while pfRegen is set: the first shield clears it, and the cleanup step
// clears the whole mask.
const pfRegenX uint16 = 3 << 13

// regenShields is gorge's "Shield" marker count on p.
func (p *perm) regenShields() int {
	if p.flags&pfRegen == 0 {
		return 0
	}
	return 1 + int(p.flags&pfRegenX>>13)
}

// regenShieldsAt is regenShields with the arena's mechRegen mask's third bit.
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
	p.flags = p.flags&^pfRegenX | pfRegen | uint16(x&3)<<13
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
		s.capFull(StallRegen) // a ninth shield: three bits hold eight (kverify-c4: Fog of Gnats' fifth; kcaps2x 4 -> 8)
	}
}

// spendRegenShield removes one shield (gorge ReplaceDestruction's
// CounterChange Shield -1).
func (s *State) spendRegenShield(sl Slot) {
	switch n := s.regenShieldsAt(sl); {
	case n <= 1:
		s.perms[sl].flags &^= pfRegen | pfRegenX
		s.mSlClr(s.k(), mechRegen, sl)
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
	if s.kw(k, sl)&kop.KwIndestructible != 0 {
		return
	}
	if s.destroyHeal(k, sl) {
		return
	}
	if k.gen != nil && k.gen.anyRepl&rbShield != 0 && s.shieldDestroy(sl) {
		return // CR 122.1c: NoRegen$ suppresses regeneration, not the shield counter (gorge effDestroy)
	}
	s.leave(k, sl, kop.ZoneGraveyard, false)
}
