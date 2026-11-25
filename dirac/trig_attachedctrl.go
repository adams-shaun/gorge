package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// attachedCtrl is kop.RefAttachedCtrl: the controller of the permanent o the
// resolving source is attached to (kop.RefAttached's object), read live; an
// object that has left reads its owner (ctrlOfObj). 255 (no player) when the
// source is attached to nothing (Contaminated Bond's blocker).
func (s *State) attachedCtrl(o Obj) uint8 {
	if o == NoObj {
		return 255
	}
	return s.ctrlOfObj(o)
}

// enchantedCtrl is kop.RefEnchantedCtrl for the entry whose source object is
// src: the controller of the permanent the source is attached to while the
// source is on the battlefield, 255 (no player) once it has left or is attached
// to nothing (gorge playerEnchantedController reads the source's live
// AttachedTo, which leaving the battlefield clears).
func (s *State) enchantedCtrl(src Obj) uint8 {
	if src == NoObj || s.objs[src].zone != kop.ZoneBattlefield {
		return 255
	}
	a := s.perms[s.objs[src].slot].attached
	if a == NoSlot || !s.occ.has(a) {
		return 255
	}
	return s.perms[a].ctrl
}
