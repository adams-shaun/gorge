package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A search that reaches outside the game: ChangeZone Origin$ Sideboard (North
// Wind Avatar) or Library,Sideboard (Turtles Forever). The pool is the
// fetching player's library (unless SrNoLibrary) then the cards of their
// sideboard, in object order, matching the filter (gorge effSearchLibrary's
// candidate walk: the library's window first, then each other origin zone,
// no card offered twice). Cold and out of line; the hot sites test one flag
// bit and call in here. The state grows by no bytes.

// DebugSideSearchLibraryOnly is the mutation switch: the sideboard is never
// offered, so a search that is met only from outside the game finds nothing
// and diverges from gorge.
var DebugSideSearchLibraryOnly bool

// sideOp reports an OpSearchEx whose pool includes the sideboard.
func sideOp(in *kop.Instr) bool {
	return in.Op == kop.OpSearchEx && in.B.N&kop.SrSideboard != 0
}

// sidePoolMax bounds the pool: a whole library and a whole sideboard.
const sidePoolMax = MaxDeck + MaxSide

// sidePool lists player p's search pool for in into buf, the library first
// (top down) unless SrNoLibrary, then the sideboard in object order, and
// returns its length.
//
//go:noinline
func (s *State) sidePool(in *kop.Instr, p uint8, buf *[sidePoolMax]Obj) int {
	n := 0
	if in.B.N&kop.SrNoLibrary == 0 {
		pl := &s.pl[p]
		n = copy(buf[:], pl.lib[pl.libLo:pl.libHi])
	}
	if DebugSideSearchLibraryOnly {
		return n
	}
	for o := Obj(0); int(o) < int(s.nobj) && n < len(buf); o++ {
		if ob := &s.objs[o]; ob.zone == kop.ZoneSideboard && ob.owner() == p {
			buf[n] = o
			n++
		}
	}
	return n
}

// sideCands is searchCands of a sideboard search: the pool's matching cards
// not yet picked, and, under DifferentNames$, no card sharing a name with one
// already picked.
//
//go:noinline
func (s *State) sideCands(k *prep, in *kop.Instr, d *Decision) {
	p := s.g.pickP
	var buf [sidePoolMax]Obj
	n := s.sidePool(in, p, &buf)
next:
	for _, o := range buf[:n] {
		if s.picksHas(o) {
			continue
		}
		if s.g.pickF&kop.SrDifferentNames != 0 {
			for i := 0; i < int(s.g.nlook); i++ {
				if s.objs[s.resArm().look[i]].card == s.objs[o].card {
					continue next
				}
			}
		}
		if s.matchCardGen(k, in.F, s.objs[o].card, p, s.res.e.ctrl, o) {
			d.add(MakeAction(AkChoose, TCode(o), 0, 0))
		}
	}
}

// sideClamp caps a sideboard search's count (startSearch): under
// DifferentNames$ at the distinct names among the pool's matching cards
// (gorge zone_search.go: max is clamped to the distinct names), and under
// Exactly$ to nothing when fewer than the requested number exist -- exactly
// that many are found or none (gorge: `cz.Exactly && max < requested`).
//
//go:noinline
func (s *State) sideClamp(k *prep, in *kop.Instr) {
	p := s.g.pickP
	var buf [sidePoolMax]Obj
	n := s.sidePool(in, p, &buf)
	var seen [sidePoolMax]CardID
	have := 0
	for _, o := range buf[:n] {
		c := s.objs[o].card
		dup := false
		if s.g.pickF&kop.SrDifferentNames != 0 {
			for i := 0; i < have; i++ {
				if seen[i] == c {
					dup = true
					break
				}
			}
		}
		if dup || !s.matchCardGen(k, in.F, c, p, s.res.e.ctrl, o) {
			continue
		}
		seen[have] = c
		have++
	}
	if int(s.g.pickLeft) > have {
		if in.B.N&kop.SrExactly != 0 {
			have = 0
		}
		s.g.pickLeft = uint8(have)
	}
}

// exactlyMayStop reports whether Done is among a search's options: an
// Exactly$ search may end only before its first card (the cards found are
// exactly the count or none), so once one is picked the rest are required.
func (s *State) exactlyMayStop(in *kop.Instr) bool {
	return in.Op != kop.OpSearchEx || in.B.N&kop.SrExactly == 0 || s.g.nlook == 0
}

// sideboardList copies player p's sideboard into list, in object order, and
// returns its length (a Library,Sideboard sweep's second zone).
//
//go:noinline
func (s *State) sideboardList(p uint8, list []Obj) int {
	n := 0
	for o := Obj(0); int(o) < int(s.nobj) && n < len(list); o++ {
		if ob := &s.objs[o]; ob.zone == kop.ZoneSideboard && ob.owner() == p {
			list[n] = o
			n++
		}
	}
	return n
}
