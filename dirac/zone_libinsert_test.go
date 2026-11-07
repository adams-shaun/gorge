package dirac

import "testing"

// CR 733.1-2: unpushSpell puts a reversed library cast back at the index
// pushBuild recorded; libInsert places at that index from the top, both
// before and after the library's top has been drawn past (libLo > 0), and
// clamps past the bottom.
func TestLibInsertRestoresIndex(t *testing.T) {
	for _, lo := range []uint8{0, 2} {
		var pl player
		pl.libLo, pl.libHi = lo, lo
		for o := Obj(1); o <= 5; o++ {
			pl.libBottom(o)
		}
		if i := pl.libIndex(4); i != 3 {
			t.Fatalf("lo %d: libIndex(4) = %d, want 3", lo, i)
		}
		pl.libRemove(4)
		pl.libInsert(4, 3)
		for i, want := range []Obj{1, 2, 3, 4, 5} {
			if got := pl.lib[int(pl.libLo)+i]; got != want {
				t.Fatalf("lo %d: lib[%d] = %d, want %d (%v)", lo, i, got, want, pl.lib[pl.libLo:pl.libHi])
			}
		}
		pl.libRemove(5)
		pl.libInsert(5, 9)
		if pl.lib[pl.libHi-1] != 5 || pl.libLen() != 5 {
			t.Fatalf("lo %d: past-the-bottom insert: %v", lo, pl.lib[pl.libLo:pl.libHi])
		}
	}
}
