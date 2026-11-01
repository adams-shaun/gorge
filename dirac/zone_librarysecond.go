package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// toLibrarySecond resumes OpToLibraryChoice with kop.LibChoiceSecond
// (AlternativeDecider$ with LibraryPosition$ 1, gorge
// zone_change.go's "second from top" / "bottom" ask). The bottom answer is
// the plain bottom move; the primary puts the object into its owner's
// library, then second from the top (gorge libraryOrderPlacementAt(1): the
// offset is min(1, the other cards left)). The carriers target permanents
// only (kgen permOrSpellMove refuses a LibraryPosition$).
//
//go:noinline
func (s *State) toLibrarySecond(k *prep, in *kop.Instr, bottom bool) {
	if bottom {
		if DebugLibposLog {
			LibposCount[LibposBottom]++
		}
		s.moveRef(k, in.Obj, kop.ZoneLibrary, true)
		return
	}
	o := s.objRefObj(in.Obj)
	if o == NoObj {
		return
	}
	s.moveRef(k, in.Obj, kop.ZoneLibrary, false)
	pl := &s.pl[s.objs[o].owner()]
	if s.objs[o].zone != kop.ZoneLibrary || pl.libLen() == 0 || pl.lib[pl.libLo] != o {
		return // a token that ceased to exist, or a replaced move
	}
	off := min(1, pl.libLen()-1)
	if DebugLibposLog {
		LibposCount[LibposSecond]++
		if pl.libLen()-1 <= 1 {
			LibposCount[LibposClamp]++
		}
	}
	if DebugLibposTop {
		off = 0 // test only: the primary goes on the top
	}
	for i := 0; i < off; i++ {
		pl.lib[int(pl.libLo)+i] = pl.lib[int(pl.libLo)+i+1]
	}
	pl.lib[int(pl.libLo)+off] = o
}

// DebugLibposTop is the lockstep mutation switch (test only): the
// "second from top" answer puts the object on the top.
var DebugLibposTop bool

// DebugLibposLog makes the paths count into LibposCount (test
// only, dirac/oracle/libpos_test.go).
var DebugLibposLog bool

// LibposCount indices.
const (
	LibposSecond  = iota // a "second from top" placement
	LibposBottom         // a LibChoiceSecond bottom answer
	LibposClamp          // a "second from top" into a library of 0 or 1 other cards
	LibposShuffle        // a Shuffle$ True move into a library (toLibraryShuffle)
)

// LibposCount is what DebugLibposLog counts.
var LibposCount [4]int
