package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// movedUnderOtherSlot reports that target ref r's object o has changed zones
// since it was targeted and another target of the resolving entry names it
// too: an earlier instruction moved it (Swirling Torrent's two modes at one
// creature), so it is a new object this move can't affect (CR 400.7,
// 608.2b). An object only this slot names keeps the plain read, so a later
// instruction can still follow its own target to the zone it sent it.
func (s *State) movedUnderOtherSlot(r kop.Ref, o Obj) bool {
	sl, ok := tgtSlot(r)
	if !ok || o == NoObj {
		return false
	}
	e := &s.res.e
	if s.objs[o].ver == e.tver[sl] {
		return false
	}
	for i := 0; i < int(e.nt); i++ {
		if i != sl && e.tgt[i] == TCode(o) {
			return true
		}
	}
	return false
}
