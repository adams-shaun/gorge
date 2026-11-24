package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// followSelfReturn keeps Self meaning the returned card for the rest of the
// resolution: an ability that returns its own source onto the battlefield and
// then acts on it (Sword of the Meek's "return it from your graveyard, then
// attach it to that creature") finds the card one version past the entry's
// srcVer, since the entry moved it; objRef would read that as a new object
// and the later Self op would do nothing. gorge's Self is the object id,
// which the return keeps, so the resolution's copy of the entry adopts the
// new version (the stack entry itself is popped when the resolution ends).
func (s *State) followSelfReturn(in *kop.Instr, o Obj) {
	e := &s.res.e
	if in.Obj != kop.RefSelf || o != e.obj || e.kind == seSpell {
		return
	}
	if ob := &s.objs[o]; ob.zone == kop.ZoneBattlefield {
		e.srcVer = ob.ver
	}
}
