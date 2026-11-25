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
	if z := s.objs[o].zone; z == kop.ZonePhased || z == kop.ZonePhasedInd {
		// a phased-out host keeps its controller (gorge's Object.Controller
		// stands while it is phased out), not its owner
		return s.perms[s.objs[o].slot].ctrl
	}
	return s.ctrlOfObj(o)
}
