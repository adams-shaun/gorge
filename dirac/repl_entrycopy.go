package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The CR 616.1 entry-order ask of a copy token (Relm's Sketching copying an
// enters-tapped land beside The Wandering Minstrel's "lands you control enter
// untapped"). gorge mints the copy outside every zone (copyTokenEnter's
// CopyToken event) and poses the order ask on the entry's MoveZone, before the
// move, to the copy's controller; the copy's own enters-tapped line is scanned
// after the battlefield's bodies (the token is in no library). The kernel
// parks the minted copy as a parked as-enters copy is (parkCopy: pending, in
// no zone) with the same ask (raGenEntryOrder, s.g.ro) and enters it from the
// answer, which enterGen reads (entryOrderEnter).
//
// Only the two-body competition is ordered here, one tap against one untap:
// the first pick decides, so no re-ask follows the entry (CR 616.1e). A longer
// competition, and a copy that enters attached to a host, stay a stall
// (StallEntryOrder, in enterGen).

// entryParkCopy parks the minted copy token t, about to enter for ctrl on its
// own, on the order ask; left is the copies after it. False when the entry
// proceeds (no ordered competition).
func (s *State) entryParkCopy(k *prep, t Obj, ctrl uint8, left int) bool {
	if k.gen == nil || !k.gen.anyEntUntap || s.res.ask != raNone || !s.entryOrderPark(k, t, ctrl, entOp) {
		return false
	}
	ro := &s.g.ro
	if bits.OnesCount8(ro.prev) != 1 || bits.OnesCount8(ro.shld) != 1 {
		ro.replHdr = replHdr{} // a longer competition re-asks after the entry: stays a stall
		return false
	}
	s.objs[t].of |= ofPending
	s.res.lastTok = t
	s.res.ask, s.res.askP = raGenEntryOrder, ro.p
	s.g.pickLeft = uint8(left)
	return true
}

// entryCopyParked reports the parked entry-order ask is a copy token's, not
// a card's move.
func (s *State) entryCopyParked() bool {
	ro := &s.g.ro
	if !ro.entHdr() || ro.pcol&entMode != entOp || ro.entObj() == NoObj {
		return false
	}
	const want = ofCopy | ofPending | ofToken
	return s.objs[ro.entObj()].of&want == want && s.objs[ro.entObj()].zone == kop.ZoneNone
}

// entryCopyAnswer takes an answer to a parked copy token's order ask: the
// pick decides the tapped bit, the copy enters, and the copies after it (and
// the targets after them) go on.
func (s *State) entryCopyAnswer(k *prep, a Action) {
	if s.entryOrderPick(k, a) == entAsk {
		return // not an option: ask again
	}
	r := &s.res
	in := &s.prog(k)[r.pc]
	t := r.lastTok
	s.objs[t].of &^= ofPending
	was := r.e.ctrl
	r.e.ctrl = s.copyCtrl(in)
	s.copyResumedEnter(k, in, t, NoSlot) // enterGen consumes the answer
	s.entryOrderClear()
	r.ask = raNone
	parked := s.copyTokens(k, in, s.objs[t].card, int(s.g.pickLeft))
	if !parked {
		parked = s.eachResume(k, in) // copy_eachtarget.go: the targets after it
	}
	if !parked {
		parked = s.nonRemResume(k, in, was, r.e.ctrl) // copy_nonremctrl.go: the owners after it
	}
	r.e.ctrl = was
	if !parked {
		s.continueRes(k, 0)
	}
}
