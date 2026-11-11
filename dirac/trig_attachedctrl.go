package dirac

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
