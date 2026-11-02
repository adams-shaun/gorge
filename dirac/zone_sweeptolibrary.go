package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A battlefield sweep into the owners' libraries, then a shuffle of each
// library that received something (Teferi, Temporal Pilgrim: "they
// shuffle each nonland permanent they control into its owner's library";
// gorge effChangeZoneAll with Shuffle$ True: the moves settle seat 0's
// battlefield first, then seat 1's, each owner's moved cards then sit on top
// of its library in that order (libraryOrderPlacement), and
// changeZoneAllShuffleLibraries shuffles the owners in order of their first
// placement, a token-only owner included: gorge records a token's placement
// too). ExtLibSweep (F the permanents swept, matched as the resolving
// controller) runs it with no remembered set.

// libSweep is ExtLibSweep.
//
//go:noinline
func (s *State) libSweep(k *prep, in *kop.Instr) {
	e := &s.res.e
	src := s.objRef(k, kop.RefSelf)
	var hit slotMask
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if s.matchPermGen(k, in.F, sl, e.ctrl, src, layerFull) {
			hit = hit.with(sl)
		}
	}
	var order [MaxPerm]Slot
	n := 0
	for p := uint8(0); p < 2; p++ {
		n = s.battlefieldOrder(p, hit.and(s.ctrlM[p]), order[:], n)
	}
	if n == 0 {
		return // gorge shuffles only a receiving library
	}
	var objs [MaxPerm]Obj
	for i := 0; i < n; i++ {
		objs[i] = s.perms[order[i]].obj
	}
	s.leaveAll(k, hit, kop.ZoneLibrary)
	first := [2]int{-1, -1} // the first placement of each owner's cards
	for i := 0; i < n; i++ {
		if p := s.objs[objs[i]].owner(); first[p&1] < 0 {
			first[p&1] = i
		}
	}
	for p := uint8(0); p < 2; p++ {
		if first[p] < 0 {
			continue
		}
		pl := &s.pl[p]
		var os [MaxPerm]Obj
		m := 0
		for i := 0; i < n; i++ {
			if o := objs[i]; s.objs[o].owner() == p && s.objs[o].zone == kop.ZoneLibrary {
				os[m] = o
				m++
			}
		}
		// the cards sit on top, the last moved first; gorge puts the first
		// moved on top
		if m > 0 && int(pl.libLo)+m <= int(pl.libHi) {
			for i := 0; i < m; i++ {
				pl.lib[int(pl.libLo)+i] = os[i]
			}
		}
	}
	for i := 0; i < n; i++ {
		p := s.objs[objs[i]].owner()
		if first[p&1] == i {
			s.shuffleLib(p)
		}
	}
}
