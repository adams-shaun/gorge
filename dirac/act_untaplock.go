package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A source-bound untap lock (OpSkipUntap C kop.SkipUntapLockStep and
// kop.SkipUntapLockAny): "it doesn't untap during its controller's untap
// step", or "it can't become untapped", "for as long as you control
// <this>" (Ty Lee, Chi Blocker; Spider-Woman, Secret Agent). gorge registers a
// Duration$ AsLongAsControl Effect replacement over the remembered target.
// continuousLive keeps it live only while its source is on the battlefield
// under the effect's controller, and expireAsLongAsControl drops it for good
// when the source changes controller. The source leaving does not drop it: a
// source that returns under the same controller (the same object, as no
// sweep reads its incarnation) makes the lock live again. ForgetOnMoved$
// Battlefield forgets the target the moment it leaves, for good.
//
// The table is a fixed four records in genState (3 B each, plus a count: 13 B
// out of rsv). A record: the target and source objects, and the controller
// that made it with ulAny set for the any-event form. An object that leaves
// is a new object (CR 400.7), so the target's departure drops its records and
// a token source's too (the object is reused); a card source's records stay
// and are read live. A fifth record stalls (StallUntapLock).

const (
	maxUntapLock = 4
	ulBytes      = maxUntapLock*3 + 1 // the table and its count: taken from rsv

	ulAny  uint8 = 1 << 7 // the record is the any-event form
	ulCtrl uint8 = ulAny - 1
)

// untapLock is one record.
type untapLock struct {
	tgt, src Obj
	ctrl     uint8 // the controlling player, with ulAny
}

// StallUntapLock is a fifth source-bound untap lock at once (genState.ul;
// act_untaplock.go).
const StallUntapLock = 155

func init() { stallNames[StallUntapLock] = "untaplock" }

// untapLockAdd is OpSkipUntap C >= kop.SkipUntapLockStep: lock the battlefield
// permanent in.Obj for as long as the resolving ability's source is under the
// resolving controller.
//
//go:noinline
func (s *State) untapLockAdd(k *prep, in *kop.Instr) {
	sl := s.objRef(k, in.Obj)
	if sl == NoSlot {
		return
	}
	r := untapLock{tgt: s.perms[sl].obj, src: s.res.e.obj, ctrl: s.res.e.ctrl & ulCtrl}
	if in.C == kop.SkipUntapLockAny {
		r.ctrl |= ulAny
	}
	for i := 0; i < int(s.g.nul); i++ {
		if s.g.ul[i] == r {
			return // one more of the same record changes nothing: both end together
		}
	}
	if int(s.g.nul) == maxUntapLock {
		s.capFull(StallUntapLock)
		return
	}
	s.g.ul[s.g.nul] = r
	s.g.nul++
	s.touchSl(sl)
}

// untapLockLive reports whether record r has its source: on the battlefield
// under the player that made it.
func (s *State) untapLockLive(r untapLock) bool {
	src := &s.objs[r.src]
	return src.zone == kop.ZoneBattlefield && s.perms[src.slot].ctrl == r.ctrl&ulCtrl
}

// untapLocked reports whether a live record locks object o; anyOnly asks for
// the any-event form (an untap outside the untap step). Callers gate on
// s.g.nul != 0.
func (s *State) untapLocked(o Obj, anyOnly bool) bool {
	for i := 0; i < int(s.g.nul); i++ {
		if r := s.g.ul[i]; r.tgt == o && (!anyOnly || r.ctrl&ulAny != 0) && s.untapLockLive(r) {
			return true
		}
	}
	return false
}

// untapLockDrop removes every record keep rejects, keeping the order.
func (s *State) untapLockDrop(drop func(r untapLock) bool) {
	n := 0
	for i := 0; i < int(s.g.nul); i++ {
		if r := s.g.ul[i]; !drop(r) {
			s.g.ul[n] = r
			n++
		}
	}
	for i := n; i < int(s.g.nul); i++ {
		s.g.ul[i] = untapLock{}
	}
	s.g.nul = uint8(n)
}

// untapLockLeft ends the records of an object that left the battlefield: o as
// the target (ForgetOnMoved$ Battlefield), and o as the source when it is a
// token, which never returns.
//
//go:noinline
func (s *State) untapLockLeft(o Obj) {
	tok := s.objs[o].of&ofToken != 0
	s.untapLockDrop(func(r untapLock) bool { return r.tgt == o || tok && r.src == o })
}

// untapLockCtrlDrop ends the records whose source is o now that its
// controller is to and the record's player is not (gorge expireAsLongAsControl).
//
//go:noinline
func (s *State) untapLockCtrlDrop(o Obj, to uint8) {
	s.untapLockDrop(func(r untapLock) bool { return r.src == o && r.ctrl&ulCtrl != to })
}
