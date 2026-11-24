package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// bounceRememberedLib is bounceRemember over a remembered set of several
// objects with Origin$ Library: every remembered library card moves to its
// owner's hand in remembered order, then each owner's library shuffles once
// when the script owes it (gorge moveDefinedLibraryObjects: the fetch list
// grouped by owner in first-appearance order, one shuffle after each owner's
// moves). The set is copied first: ForgetChanged$ shrinks it as cards move.
//
//go:noinline
func (s *State) bounceRememberedLib(k *prep, in *kop.Instr) {
	var objs [maxRem]Obj
	n := copy(objs[:], s.g.rem[:s.g.nrem])
	var owners [maxRem]uint8
	no := 0
	for _, o := range objs[:n] {
		if s.objs[o].zone != kop.ZoneLibrary {
			continue
		}
		p, seen := s.objs[o].owner(), false
		for _, q := range owners[:no] {
			seen = seen || q == p
		}
		if !seen {
			owners[no] = p
			no++
		}
	}
	for _, p := range owners[:no] {
		moved := false
		for _, o := range objs[:n] {
			if s.objs[o].owner() != p || s.objs[o].zone != kop.ZoneLibrary {
				continue
			}
			v := s.objs[o].ver
			s.moveRefObj(k, in.Obj, o, kop.ZoneHand, false)
			if s.objs[o].ver == v {
				continue // nothing moved
			}
			moved = true
			if in.C&kop.MvRemember != 0 {
				s.remember(o)
			}
			if in.C&kop.MvForgetChanged != 0 {
				s.forgetChanged(o) // zone_forget.go
			}
			if in.C&kop.MvLKI != 0 {
				s.markRemLKI(o)
			}
		}
		if moved && in.C&kop.BncShuffleOwed != 0 {
			s.shuffleLib(p)
		}
	}
}
