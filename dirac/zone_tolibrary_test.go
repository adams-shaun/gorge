package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestToLibraryShuffle: OpToLibrary kop.LibTopShuffle puts the object on
// top of its owner's library, then shuffles that library (the rng draws).
func TestToLibraryShuffle(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 1}, {Name: "Plains", Count: 8}})
	fc := id["Valley Floodcaller"]
	a := sc.perm(fc, 1)
	sc.s.res.e.ctrl, sc.s.res.e.obj, sc.s.res.e.card = 0, a, fc
	sc.s.res.e.tgt[0], sc.s.res.e.nt, sc.s.res.e.tver[0] = TCode(a), 1, sc.s.objs[a].ver
	pl := &sc.s.pl[1]
	n0, d0 := pl.libLen(), sc.s.rng.Draws
	in := kop.Instr{Op: kop.OpToLibrary, Obj: kop.RefTarget0, C: kop.LibTopShuffle}
	sc.s.exec(sc.k, &in)
	if sc.s.objs[a].zone != kop.ZoneLibrary || pl.libLen() != n0+1 || sc.s.rng.Draws == d0 {
		t.Fatalf("zone %d, library %d -> %d, draws %d: want it in its owner's library, shuffled", sc.s.objs[a].zone, n0, pl.libLen(), sc.s.rng.Draws-d0)
	}
}

// TestLibTopInOrder: a ChangeZoneAll onto the library top leaves the first
// moved card on top, as gorge's libraryOrderPlacement does.
func TestLibTopInOrder(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	pl := &s.pl[0]
	a, b := pl.lib[pl.libLo], pl.lib[pl.libLo+1]
	fromA := s.detach(a)
	fromB := s.detach(b)
	s.libTopInOrder([]Obj{a, b}, []kop.Zone{fromA, fromB})
	if pl.lib[pl.libLo] != a || pl.lib[pl.libLo+1] != b {
		t.Fatalf("library top %v %v, want %v %v", pl.lib[pl.libLo], pl.lib[pl.libLo+1], a, b)
	}
}
