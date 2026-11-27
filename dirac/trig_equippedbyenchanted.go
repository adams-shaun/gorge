package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// RefEquippedByEnchanted is gorge's Defined$ Valid Card.EquippedByEnchanted:
// the permanent the source Aura's enchanted Equipment is attached to. The
// source is the Aura, its attached object is the Equipment, and the
// Equipment's own attached object is the creature the filter names
// (effects/filter_attachment.go attachedToArg). It names at most one object.
//
//go:noinline
func (s *State) equippedByEnchanted(o Obj) Obj {
	if o == NoObj || int(o) >= len(s.objs) {
		return NoObj
	}
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield {
		return NoObj
	}
	eq := s.perms[ob.slot].attached // the Aura's enchanted Equipment
	if eq == NoSlot || !s.occ.has(eq) {
		return NoObj
	}
	h := s.perms[eq].attached // the Equipment's equipped creature
	if h == NoSlot || !s.occ.has(h) {
		return NoObj
	}
	return s.perms[h].obj
}
