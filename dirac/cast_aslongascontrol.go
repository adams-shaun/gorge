package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// a Duration$ AsLongAsControl grant also reads
// the source's controller (CR 611.2b). gorge continuousLive treats the
// registration as not live while the source's controller is not the effect's
// controller (r.ctrl), and expireAsLongAsControl deletes it on the source's
// ControlChange when it no longer controls the source: gone for good, it does
// not come back when control returns (Act of Treason's end-of-turn return) or
// when the source leaves and returns. A grant made while the source is already
// under another controller is only not live, and is live again once control
// comes back. 0 B: the controller is the record's ctrl.

// DebugMayWhileSrcIgnoreCtrl makes DurWhileSrcMay ignore the source's
// controller (mutation testing only: the lockstep must catch it).
var DebugMayWhileSrcIgnoreCtrl bool

// maySrcCtrlOK reports whether the battlefield source src is controlled by
// the player r grants to.
func (s *State) maySrcCtrlOK(r mayPlay, src *object) bool {
	return DebugMayWhileSrcIgnoreCtrl || s.perms[src.slot].ctrl == r.ctrl
}

// mayWhileSrcCtrlDrop drops every DurWhileSrcMay record whose source is the
// object o (a token: the same incarnation) now that o's controller is to and
// the record's player is not (gorge expireAsLongAsControl). The records keep
// their order; no caller holds an index into s.g.may across a control change.
//
//go:noinline
func (s *State) mayWhileSrcCtrlDrop(o Obj, to uint8) {
	if DebugMayWhileSrcIgnoreCtrl {
		return
	}
	n := 0
	for i := 0; i < int(s.g.nmay); i++ {
		r := *s.mayAt(i)
		if r.dur() == kop.DurWhileSrcMay && r.heldObj() == o && r.ctrl != to &&
			(s.objs[o].of&ofToken == 0 || s.objs[o].ver == r.heldVer()) {
			continue
		}
		*s.mayAt(n) = r
		n++
	}
	s.g.nmay = uint8(n)
}
