package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestObjSetsOrderAndGC checks the one cold object-set store (zone_objsets.go):
// insertion order over interleaved kinds, the two GC arms (a stale set-kind
// record goes, a flagged exiled-with record stays), and a kinds-mask forget.
func TestObjSetsOrderAndGC(t *testing.T) {
	sc := srcMemScen(t)
	s := sc.s
	a := sc.perm(vanilla.GrizzlyBears, 0)
	x, y := sc.perm(vanilla.GrizzlyBears, 1), sc.perm(vanilla.GrizzlyBears, 1)
	ver := s.objs[a].ver

	// interleaved kinds on one source: insertion order is the store's, and
	// each kind's reader walks it in that order
	s.osAdd(osRem, a, x)
	s.exwPut(x, a, ver, exwFwd, -1)
	s.osAdd(osRem, a, y)
	s.osAdd(osImp, a, x)

	if !s.osHas(osRem, a, x, ver) || !s.osHas(osRem, a, y, ver) {
		t.Fatal("osHas lost a remembered record")
	}
	if !s.osHas(osExw, a, x, ver) {
		t.Fatal("osHas lost the exiled-with record")
	}
	if !s.osHas(osImp, a, x, ver) {
		t.Fatal("osHas lost the imprinted record")
	}

	var seen []Obj
	s.osEach(osMRem, a, nil, func(r *osRec) { seen = append(seen, r.o) })
	if len(seen) != 2 || seen[0] != x || seen[1] != y {
		t.Fatalf("osEach order %v, want [%d %d]", seen, x, y)
	}

	// osGC's two arms: the source departed (ver moved), so its stale set-kind
	// records go, while the flagged exiled-with record stays (its readers'
	// rules, not the GC's, decide its life)
	s.objs[a].ver += 5
	s.osGC()
	if s.osAnyOf(osMRem) || s.osAnyOf(osMImp) {
		t.Fatalf("osGC kept a stale set record: %v", srcMemRecs(s))
	}
	if !s.osHas(osExw, a, x, ver) {
		t.Fatal("osGC dropped a flagged exiled-with record")
	}

	// the flags cleared, the record is unreadable and the GC takes it
	for i := 0; i < s.osLen(); i++ {
		if rec := &s.cold.objSets[i]; rec.osKind() == osExw {
			rec.kind = osExw
		}
	}
	s.osGC()
	if s.osAnyOf(osMExw) {
		t.Fatalf("osGC kept a flagless exiled-with record: %v", srcMemRecs(s))
	}

	// osForget empties a set by its kinds mask
	s.objs[a].ver = ver
	s.osAdd(osRem, a, x)
	s.osAdd(osRem, a, y)
	s.osForget(x, a, osMRem)
	if s.osHas(osRem, a, x, ver) || !s.osHas(osRem, a, y, ver) {
		t.Fatalf("osForget left %v", srcMemRecs(s))
	}
}
