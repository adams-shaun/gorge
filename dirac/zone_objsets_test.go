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
	s.osEach(osMRem, a, nil, func(o Obj) { seen = append(seen, o) })
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
	for i := range s.cold.objSets {
		rec := &s.cold.objSets[i]
		if rec.kind == 0 {
			break
		}
		if rec.osKind() == osExw {
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

	// full means stall at each capacity: 96 when the table keeps both stores'
	// features, 64 when it keeps one (objSetsCap)
	saved := s.cold.objSets
	full := make([]osRec, maxObjSets)
	for _, capacity := range []int{maxObjSetsOne, maxObjSets} {
		s.g.stalled = 0
		s.cold.objSets = full[:capacity]
		for i := range full {
			full[i] = osRec{}
		}
		for i := 0; i < capacity; i++ {
			s.osAdd(osRem, a, Obj(i))
		}
		if s.Stalled() || s.osLen() != capacity {
			t.Fatalf("cap %d: filling the store: len %d stalled %v", capacity, s.osLen(), s.Stalled())
		}
		s.exwPut(Obj(capacity), a, ver, exwFwd, -1) // the exw writer stalls on the same full store
		if s.StallReason() != StallObjSets || StallName(StallObjSets) != "objsets" {
			t.Fatalf("cap %d: record %d: stall %q, want objsets", capacity, capacity+1, StallName(s.StallReason()))
		}
	}
	s.cold.objSets = saved
}

// TestObjSetsCapacity pins the sizing rule: a table that kept both old stores
// gets 96 records, one that kept a single store keeps its 64, neither is 0.
func TestObjSetsCapacity(t *testing.T) {
	for _, c := range []struct {
		g    genPrep
		want int
	}{
		{genPrep{anySrcMem: true, anyExW: true}, maxObjSets},
		{genPrep{anySrcMem: true, anyFaceExile: true}, maxObjSets},
		{genPrep{anySrcMem: true}, maxObjSetsOne},
		{genPrep{anyExW: true}, maxObjSetsOne},
		{genPrep{anyFaceExile: true}, maxObjSetsOne},
		{genPrep{}, 0},
	} {
		if got := objSetsCap(&c.g); got != c.want {
			t.Errorf("%+v: capacity %d, want %d", c.g, got, c.want)
		}
	}
}
