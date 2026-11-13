package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Combat accessors for dirac/search/hbot (read-only, no allocation).

// HBotBlockPair reports whether battlefield object b may block attacking
// object a as gorge's KBlockers decision offers the pair -- b an untapped
// creature that can block, a attacking, the block legal (canBlock), a not
// a menace attacker fewer than two of the defender's creatures could
// block -- whether or not b has already been declared as a blocker this
// step (the declaration's original pair set).
func (s *State) HBotBlockPair(b, a Obj) bool {
	ob, oa := &s.objs[b], &s.objs[a]
	if ob.zone != kop.ZoneBattlefield || oa.zone != kop.ZoneBattlefield || ob.slot == NoSlot || oa.slot == NoSlot {
		return false
	}
	k := s.k()
	bs, as := ob.slot, oa.slot
	p := &s.perms[bs]
	if p.flags&pfTapped != 0 || !s.isCreature(k, bs) || s.kw(k, bs)&kop.KwCantBlock != 0 {
		return false
	}
	if s.perms[as].flags&pfAttacking == 0 {
		return false
	}
	return s.canBlock(k, bs, as) && !s.menaceImpossible(k, as)
}

// HBotAttackTarget is battlefield object o's declared attack target
// (PlayerTarget* or a planeswalker object) and whether it is attacking.
func (s *State) HBotAttackTarget(o Obj) (uint8, bool) {
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield || ob.slot == NoSlot || s.perms[ob.slot].flags&pfAttacking == 0 {
		return uint8(NoTarget), false
	}
	return s.perms[ob.slot].atk, true
}

// HBotBlocking is the attacker battlefield object o is declared blocking
// (NoObj none).
func (s *State) HBotBlocking(o Obj) Obj {
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield || ob.slot == NoSlot {
		return NoObj
	}
	pm := &s.perms[ob.slot]
	if pm.flags&pfBlocking == 0 {
		return NoObj
	}
	if a := s.blockProjected(ob.slot); a != NoSlot {
		return s.perms[a].obj
	}
	return NoObj
}

// HBotIsPlaneswalker reports whether battlefield object o is a planeswalker
// now (the kernel's attackable-defender test).
func (s *State) HBotIsPlaneswalker(o Obj) bool {
	ob := &s.objs[o]
	return ob.zone == kop.ZoneBattlefield && ob.slot != NoSlot && s.typesAt(s.k(), ob.slot)&kop.TPlaneswalker != 0
}

// HBotPermFlags is battlefield object o's projection flag bits
// (ProjTapped, ProjAttacking, ProjBlocking, ProjSick) without the rest of
// Project's per-object work (the board census reads only these).
func (s *State) HBotPermFlags(o Obj) uint8 {
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield || ob.slot == NoSlot {
		return 0
	}
	pm := &s.perms[ob.slot]
	var f uint8
	if pm.flags&pfTapped != 0 {
		f |= ProjTapped
	}
	if pm.flags&pfAttacking != 0 {
		f |= ProjAttacking
	}
	if pm.flags&pfBlocking != 0 {
		f |= ProjBlocking
	}
	if pm.flags&pfSick != 0 {
		f |= ProjSick
	}
	return f
}
