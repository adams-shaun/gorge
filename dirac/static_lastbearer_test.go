package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// CR 608.2h: clearing a superseded AttachedTo record for a departed
// attachment must leave the pad's EMPTY sentinel, not a zero record. Zero
// names object 0 (seat 0's first deck card), so lastBearerObj(0) would read it
// as attached to itself, and the obj == NoObj free-slot test could never
// reclaim it.
func TestLastBearClearLeavesNoPhantomRecord(t *testing.T) {
	var s State
	// Object 0 is a real card. The pad's entries start empty, as a new game
	// lays them out (one per seat slot), so the insertion below reclaims the
	// first and leaves the cleared versioned record in place.
	for i := range s.g.attLKI {
		s.g.attLKI[i] = attLKIRec{obj: NoObj, host: NoObj}
	}
	s.g.attLKI[1] = attLKIRec{obj: 5, host: 42, ver: 7}
	s.objs[42].zone = kop.ZoneGraveyard // the old bearer had already left
	s.recordLastBearLKI(5, 42)
	if got := s.lastBearerObj(0); got != NoObj {
		t.Fatalf("lastBearerObj(0) = %d, want NoObj: a zero record names the real object 0", got)
	}
	if got := s.lastBearerObj(5); got != 42 {
		t.Fatalf("lastBearerObj(5) = %d, want 42: the newest departure must survive the clear", got)
	}
}
