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
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		n = s.battlefieldOrder(p, hit.and(s.ctrlM[p]), order[:], n)
	}
	var objs [MaxPerm]Obj
	if pre := &s.g.pre; pre.kind == akCmdZone && (pre.state == preCmdZoneMove || pre.state == preCmdZoneCmd) {
		// the answer's re-run: the parked commander is the only matching
		// permanent still on the battlefield. The earlier ones are already on
		// top of their libraries, so the placement order and the shuffle's
		// owners are the sweep this instruction saved before the park; the
		// placement and shuffle below then run once, after the answer.
		n = int(s.res.left)
		if n == 0 {
			s.stall(StallLowering) // a commander park with no saved sweep
			return
		}
		copy(objs[:n], s.res.arr[:n])
	} else {
		if n == 0 {
			return // gorge shuffles only a receiving library
		}
		for i := 0; i < n; i++ {
			objs[i] = s.perms[order[i]].obj
		}
		copy(s.res.arr[:n], objs[:n])
		s.res.left = uint8(n)
	}
	s.leaveAll(k, hit, kop.ZoneLibrary)
	if s.res.ask == raGenCmdZone {
		return // the commander's CR 903.9b answer finishes the placement and shuffle
	}
	first := [2]int{-1, -1} // the first placement of each owner's cards
	for i := 0; i < n; i++ {
		if p := s.objs[objs[i]].owner(); first[SeatIx(p)] < 0 {
			first[SeatIx(p)] = i
		}
	}
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
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
		if first[SeatIx(p)] == i {
			s.shuffleLib(p)
		}
	}
}
