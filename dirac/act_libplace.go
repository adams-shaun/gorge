package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The owner's CR 401.4 arrangement of cards one effect puts at a library
// position together (kop.ExtLibPlace; gorge effects.libraryPlacementOrder). The
// move op in between places the cards in its own order -- the first moved
// nearest the top of its group, at the top or at the bottom -- which is also
// gorge's answer when nothing is asked. The arm op counts, per owner, the
// cards placeFromK puts into a library from then on (res.plcN, owners in
// first-placed order in res.plcOrd); the ask op then poses one ChArrange per
// owner with two or more of them (Min == Max == the count, the first pick
// nearest the top), asked of the OWNER, and reorders that group in place: the
// cards are the top (or bottom) n of the owner's library, since nothing else
// is placed between the arm and the ask.
//
// gorge's MoveZone appends every moved card at the BOTTOM, and a top placement
// (libraryOrderPlacement) happens only after the owner's answer; the ask is
// posed with the cards still there, and the lockstep compares library
// positions at the ask. So a top arm lifts each owner's group to the bottom
// (the move order kept) as the ask op begins, and an owner's turn ends with
// the group put on top in the answered order, owners in first-placed order
// (an owner with one card asks nothing and is placed back on top at its turn).

// libTally is the placement tally: arm 0 off, 1 the counted placements go to
// the top, 2 to the bottom; n the cards each owner has received since the arm
// and ord the owners in first-placed order, two bits each. It lives in the
// resolution's scratch arm (armRes), so State grows by nothing.
type libTally struct {
	arm, ord uint8
	n        [MaxSeats]uint8
}

// libPlaceExt is OpExt ExtLibPlace; false: an ask is posed.
func (s *State) libPlaceExt(k *prep, in *kop.Instr) bool {
	switch in.C {
	case kop.PlaceArmTop, kop.PlaceArmBottom:
		s.resArm().plc = libTally{arm: uint8(in.C) + 1}
		return true
	}
	t := &s.resArm().plc
	if t.arm == 1 {
		for i := 0; i < s.libPlaceOwners(); i++ {
			seat := t.ord >> (2 * i) & 3
			s.libPlaceLift(seat, s.libPlaceN(seat))
		}
	}
	s.res.left = 0
	return s.libPlaceNext()
}

// libPlaceOwners is how many owners the tally has seen.
func (s *State) libPlaceOwners() int {
	n := 0
	for _, c := range s.resArm().plc.n {
		if c != 0 {
			n++
		}
	}
	return n
}

// libPlaceN is the cards of seat's group still in its library.
func (s *State) libPlaceN(seat uint8) int {
	return min(int(s.resArm().plc.n[seat]), s.pl[seat].libLen())
}

// libPlaceNote counts a card put into owner's library while the arm is on.
func (s *State) libPlaceNote(owner uint8) {
	t := &s.resArm().plc
	if t.n[owner] == 0 {
		t.ord |= owner << (2 * s.libPlaceOwners())
	}
	if t.n[owner] < 255 {
		t.n[owner]++
	}
}

// libPlaceLift moves the top n cards of seat's library, in order, to the bottom.
func (s *State) libPlaceLift(seat uint8, n int) {
	pl := &s.pl[seat]
	lo, hi := int(pl.libLo), int(pl.libHi)
	var group [MaxDeck]Obj
	copy(group[:n], pl.lib[lo:lo+n])
	copy(pl.lib[lo:hi-n], pl.lib[lo+n:hi])
	copy(pl.lib[hi-n:hi], group[:n])
}

// libPlaceSettle ends seat's turn: the n cards at the bottom of its library are
// put in order there (a bottom arm) or on top (a top arm).
func (s *State) libPlaceSettle(seat uint8, n int, order []Obj) {
	pl := &s.pl[seat]
	lo, hi := int(pl.libLo), int(pl.libHi)
	if s.resArm().plc.arm == 2 {
		copy(pl.lib[hi-n:hi], order)
		return
	}
	var rest [MaxDeck]Obj
	m := hi - lo - n
	copy(rest[:m], pl.lib[lo:lo+m])
	copy(pl.lib[lo:lo+n], order)
	copy(pl.lib[lo+n:hi], rest[:m])
}

// libPlaceNext goes on with the owners after the res.left done: it poses the
// ask for the next one that received two or more cards, and settles the ones
// that received one; true when none is left and the tally has ended.
func (s *State) libPlaceNext() bool {
	r, t := &s.res, &s.resArm().plc
	owners := s.libPlaceOwners()
	for int(r.left) < owners {
		seat := t.ord >> (2 * r.left) & 3
		r.left++
		n := s.libPlaceN(seat)
		if n < 2 {
			if n == 1 {
				s.libPlaceSettle(seat, 1, []Obj{s.pl[seat].lib[int(s.pl[seat].libHi)-1]})
			}
			continue
		}
		r.peekN, r.peekI = uint8(n), 0
		r.ask, r.askP = raGenLibPlace, seat
		return false
	}
	*t, r.left = libTally{}, 0
	return true
}

// libPlaceOptions lists the owner's cards not yet ordered, in the order
// gorge offers them: the move order, which the group at the bottom keeps.
func (s *State) libPlaceOptions(d *Decision) {
	d.Sub = ChArrange
	r := &s.res
	pl := &s.pl[r.askP]
	lo := int(pl.libHi) - int(r.peekN)
	for i := 0; i < int(r.peekN); i++ {
		o := pl.lib[lo+i]
		picked := false
		for _, p := range r.arr[:r.peekI] {
			picked = picked || p == o
		}
		if !picked {
			d.add(MakeAction(AkChoose, uint8(o), 0, 0))
		}
	}
}

// libPlaceAnswer records a pick (the next card down from the top of the group);
// the last one settles the owner's group in the picked order and goes on to the
// next owner.
func (s *State) libPlaceAnswer(k *prep, a Action) {
	r := &s.res
	r.arr[r.peekI] = Obj(a.Source())
	r.peekI++
	if r.peekI < r.peekN {
		return
	}
	s.libPlaceSettle(r.askP, int(r.peekN), r.arr[:r.peekN])
	r.ask, r.peekN, r.peekI = raNone, 0, 0
	if s.libPlaceNext() {
		s.continueRes(k, 0)
	}
}
