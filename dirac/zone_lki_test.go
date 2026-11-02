package dirac

import (
	"testing"
	"unsafe"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestLKIElementSizes logs the element sizes.
func TestLKIElementSizes(t *testing.T) {
	var g genState
	t.Logf("State %d lkiRec %d pool %d stackEntry %d pendingTrig %d usedRec %d dpLKI %d",
		unsafe.Sizeof(State{}), unsafe.Sizeof(lkiRec{}), unsafe.Sizeof(g.lki), unsafe.Sizeof(stackEntry{}),
		unsafe.Sizeof(pendingTrig{}), unsafe.Sizeof(usedRec{}), unsafe.Sizeof(g.dpLKI))
}

// TestLKIPoolShareAndFree: snapshots of one unchanged object share a record;
// a record no live holder names is reused; a full pool stalls (StallLKI).
func TestLKIPoolShareAndFree(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Burglar Rat", Count: 1}, {Name: "Swamp", Count: 8}})
	rat := sc.perm(id["Burglar Rat"], 0)
	s := sc.s
	a := s.snapLKI(rat)
	s.res.e.li = a
	if b := s.snapLKI(rat); b != a {
		t.Fatalf("same (obj, ver) snapshot got %d, want shared %d", b, a)
	}
	s.res.e.li = 0
	if b := s.snapLKI(rat); b != 1 {
		t.Fatalf("freed record not reused: got %d", b)
	}
	for i := 0; i < maxLKI; i++ {
		s.pend[i].li = uint8(i + 1)
		s.g.lki[i].power = int16(100 + i)
	}
	s.npend = maxLKI
	if b := s.snapLKI(rat); b != 0 || s.StallReason() != StallLKI {
		t.Fatalf("full pool: idx %d stall %d, want 0 and StallLKI", b, s.StallReason())
	}
}
