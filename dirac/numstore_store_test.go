package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestNumStoreRecords: the number store's write, overwrite, delete-on-default,
// liveness and collection arms (act_storenum.go).
func TestNumStoreRecords(t *testing.T) {
	sc := srcMemScen(t)
	s := sc.s
	a, b := sc.perm(vanilla.GrizzlyBears, 0), sc.perm(vanilla.GrizzlyBears, 1)

	read := func(o Obj, name int) (int, bool) { return s.numRead(o, name) }
	if _, ok := read(a, 0); ok {
		t.Fatal("a fresh store holds a record")
	}
	s.numStore(a, 0, 3, true, 0)
	s.numStore(a, 1, -7, false, 0)
	s.numStore(b, 0, 9, true, 0)
	if v, ok := read(a, 0); !ok || v != 3 {
		t.Fatalf("a/0 = %d,%v, want 3", v, ok)
	}
	if v, ok := read(a, 1); !ok || v != -7 {
		t.Fatalf("a/1 = %d,%v, want -7 (negative values survive the int16 slot)", v, ok)
	}
	if v, _ := read(b, 0); v != 9 {
		t.Fatalf("b/0 = %d, want 9: records are per carrier", v)
	}

	// last write wins, in place
	n := s.osLen()
	s.numStore(a, 0, 4, true, 0)
	if v, _ := read(a, 0); v != 4 || s.osLen() != n {
		t.Fatalf("overwrite: a/0 = %d, %d records (was %d)", v, s.osLen(), n)
	}

	// a write of the printed default deletes the record; the same write with
	// no delete flag keeps it (a dynamic printed body)
	s.numStore(a, 0, 0, true, 0)
	if _, ok := read(a, 0); ok || s.osLen() != n-1 {
		t.Fatalf("delete-on-default left a record (%d records, was %d)", s.osLen(), n)
	}
	s.numStore(a, 1, 0, false, 0)
	if v, ok := read(a, 1); !ok || v != 0 {
		t.Fatalf("a no-delete write of 0 = %d,%v, want a kept 0", v, ok)
	}
	s.numStore(a, 2, 0, true, 0)
	if _, ok := read(a, 2); ok {
		t.Fatal("a delete-flagged default write with no record stored one")
	}

	// only a departure from the battlefield ends the record (CR 400.7;
	// gorge's wasBattlefield reset): a zone change elsewhere keeps it, and a
	// write after the departure persists on the departed object
	s.objs[a].zone = kop.ZoneGraveyard
	if v, ok := read(a, 1); !ok || v != 0 {
		t.Fatalf("a card in the graveyard lost its number: %d,%v", v, ok)
	}
	s.numClear(b)
	if _, ok := read(b, 0); ok {
		t.Fatal("a departure left the number")
	}
	if v, ok := read(a, 1); !ok || v != 0 {
		t.Fatalf("a departure of another object cleared a/1: %d,%v", v, ok)
	}
	st := &NumStoreStats{}
	DebugNumStoreStats = st
	defer func() { DebugNumStoreStats = nil }()
	s.numStore(a, 1, 8, false, 0)
	if v, _ := read(a, 1); v != 8 {
		t.Fatalf("a write from a card off the battlefield = %d, want 8 (it persists as gorge's does)", v)
	}
	s.numClear(a)
	if _, ok := read(a, 1); ok || st.Cleared != 1 {
		t.Fatalf("numClear: a/1 still reads (%v), cleared %d, want 1", ok, st.Cleared)
	}
	// a token off the battlefield takes no write: it ceases before a reader
	// could see it and its slot is reused
	s.objs[b].of |= ofToken
	s.objs[b].zone = kop.ZoneGraveyard
	s.numStore(b, 0, 5, true, 0)
	if _, ok := read(b, 0); ok || st.Dropped != 1 {
		t.Fatalf("a departed token's write: dropped %d", st.Dropped)
	}
}

// TestNumStoreSaturateAndFull: a value past int16 saturates and stalls, and a
// full store stalls instead of writing.
func TestNumStoreSaturateAndFull(t *testing.T) {
	sc := srcMemScen(t)
	s := sc.s
	a := sc.perm(vanilla.GrizzlyBears, 0)
	st := &NumStoreStats{}
	DebugNumStoreStats = st
	defer func() { DebugNumStoreStats = nil }()

	s.numStore(a, 0, 40000, true, 0)
	if v, _ := s.numRead(a, 0); v != 32767 || st.Saturated != 1 || s.StallReason() != StallObjSets {
		t.Fatalf("saturate: read %d, saturated %d, stall %q", v, st.Saturated, StallName(s.StallReason()))
	}
	s.g.stalled = 0
	s.numStore(a, 1, -40000, true, 0)
	if v, _ := s.numRead(a, 1); v != -32768 {
		t.Fatalf("negative saturation read %d", v)
	}

	s.g.stalled = 0
	saved := s.cold.objSets
	defer func() { s.cold.objSets = saved }()
	s.cold.objSets = make([]osRec, 3)
	for name := 0; name < 3; name++ {
		s.numStore(a, name, 1, true, 0)
	}
	if s.Stalled() {
		t.Fatal("stalled before the store was full")
	}
	s.numStore(a, 3, 1, true, 0)
	if s.StallReason() != StallObjSets {
		t.Fatalf("a write to a full store: stall %q, want objsets", StallName(s.StallReason()))
	}
	if _, ok := s.numRead(a, 3); ok {
		t.Fatal("a write to a full store was kept")
	}
}

// TestNumStorePacksC: kop.StoreNumC round-trips, a negative printed default too.
func TestNumStorePacksC(t *testing.T) {
	for _, c := range []struct {
		name int
		del  bool
		def  int
	}{{0, false, 0}, {14, true, 0}, {3, true, -1}, {7, true, 1023}, {7, true, -1024}} {
		n, d, f := kop.StoreNumUnpack(kop.StoreNumC(c.name, c.del, c.def))
		if n != c.name || d != c.del || (c.del && f != c.def) {
			t.Errorf("%+v unpacks to %d %v %d", c, n, d, f)
		}
	}
}
