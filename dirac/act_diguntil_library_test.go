package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// digLibScen stocks player 0's library top to bottom with the given vanilla
// cards (a plain Plains is a land, a Grizzly Bears a creature).
func digLibScen(t *testing.T, cs ...CardID) (*scen, []Obj) {
	t.Helper()
	sc := newScen(t, Config{NoElide: true})
	pl := &sc.s.pl[0]
	objs := make([]Obj, len(cs))
	for i, c := range cs {
		objs[i] = sc.obj(c, 0)
		sc.s.objs[objs[i]].zone = kop.ZoneLibrary
	}
	for i := len(objs) - 1; i >= 0; i-- {
		pl.libTop(objs[i])
	}
	sc.s.res.e = stackEntry{kind: seSpell, ctrl: 0, trigObj: NoObj}
	return sc, objs
}

func libOrder(s *State) []Obj {
	pl := &s.pl[0]
	return append([]Obj(nil), pl.lib[pl.libLo:pl.libHi]...)
}

// TestDigUntilRevealedLibraryStaysInPlace: a position-less library reveal
// leaves every scanned card where it is (gorge effDigUntil's stay arm: no
// reorder, no rng draw), found cards included when they go with the revealed
// pile.
func TestDigUntilRevealedLibraryStaysInPlace(t *testing.T) {
	sc, objs := digLibScen(t, v.Plains, v.GrizzlyBears, v.Plains, v.GrizzlyBears, v.Plains)
	before := libOrder(sc.s)
	draws := sc.s.rng.Draws
	// precondition: both bears are inside the scan window (the first four
	// cards) and the window is smaller than the library
	if len(before) <= 5 || before[0] != objs[0] {
		t.Fatalf("library %d cards, top %d", len(before), before[0])
	}
	in := kop.Instr{
		Op:  kop.OpDigUntil,
		Obj: kop.RefController,
		A:   kop.Const(2),
		C:   int16(kop.ZoneLibrary) | int16(int(kop.ZoneLibrary)|kop.DuFoundWithRevealed)<<3,
		F:   kop.FilterID(1),
	}
	sc.s.execDigUntil(sc.k, &in, false)
	for i, o := range objs {
		if z := sc.s.objs[o].zone; z != kop.ZoneLibrary {
			t.Fatalf("card %d zone %d, want the library", i, z)
		}
	}
	got := libOrder(sc.s)
	for i := range before {
		if got[i] != before[i] {
			t.Fatalf("library order %v, want the unchanged %v", got, before)
		}
	}
	if sc.s.rng.Draws != draws {
		t.Fatalf("RNG draws %d -> %d, want none", draws, sc.s.rng.Draws)
	}
}

// TestDigUntilRevealedLibraryRandomWindow: with the found card in exile and
// the revealed rest in the library under RandomOrder and no position, the
// rest is neither shuffled nor bottomed and no rng is drawn.
func TestDigUntilRevealedLibraryRandomWindow(t *testing.T) {
	sc, objs := digLibScen(t, v.Plains, v.GrizzlyBears, v.Plains, v.Plains)
	before := len(libOrder(sc.s))
	draws := sc.s.rng.Draws
	in := kop.Instr{
		Op:  kop.OpDigUntil,
		Obj: kop.RefController,
		A:   kop.Const(1),
		C:   int16(kop.ZoneExile) | int16(int(kop.ZoneLibrary)|kop.DuRandomOrder)<<3,
		F:   kop.FilterID(1),
	}
	// precondition: the bear is a creature and the first land is scanned
	// before it (so the revealed pile is not empty)
	if !sc.s.matchCardGen(sc.k, in.F, sc.s.objs[objs[1]].card, 0, 0, NoObj) || sc.s.matchCardGen(sc.k, in.F, sc.s.objs[objs[0]].card, 0, 0, NoObj) {
		t.Fatal("filter does not split creature from land")
	}
	sc.s.execDigUntil(sc.k, &in, false)
	if z := sc.s.objs[objs[1]].zone; z != kop.ZoneExile {
		t.Fatalf("bear zone %d, want exile", z)
	}
	want := []Obj{objs[0], objs[2], objs[3]}
	got := libOrder(sc.s)
	if len(got) != before-1 {
		t.Fatalf("library %d cards, want %d", len(got), before-1)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("library %v, want %v", got, want)
		}
	}
	if sc.s.rng.Draws != draws {
		t.Fatalf("RNG draws %d -> %d, want none", draws, sc.s.rng.Draws)
	}
}
