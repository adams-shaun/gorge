package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// trigObjMoved: the trigger's object (another card than the source) is no
// longer the object that triggered (CR 400.7; gorge every zone
// change makes a new object). A card that died while snapshot on the
// battlefield is the same object only at the version one past the snapshot
// (the death); without a snapshot only its zone is checked.
func (s *State) trigObjMoved(e *stackEntry, ob *object) bool {
	if ob.zone != kop.ZoneGraveyard {
		return true
	}
	return e.li != 0 && ob.ver != s.lk(e.li).ver+1
}
