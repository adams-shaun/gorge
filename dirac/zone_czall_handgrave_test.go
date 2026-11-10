package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// czLibAll is seat p's whole library, top down.
func czLibAll(s *State, p uint8) []Obj {
	pl := &s.pl[p]
	return slices.Clone(pl.lib[pl.libLo:pl.libHi])
}

// TestChangeZoneAllHandGraveOrder: CzHandGrave sweeps every seat's hand before
// its graveyard (gorge's zone-major walk of Origin$ Hand,Graveyard), so the
// hand cards sit above the graveyard cards in the library.
func TestChangeZoneAllHandGraveOrder(t *testing.T) {
	sc, plains, in := newWeftScen(t)
	s := sc.s
	g := sc.grave(plains, 0, 2)
	h := []Obj{sc.hand(plains, 0), sc.hand(plains, 0)}
	in.B = kop.Const(kop.CzHandGrave)
	s.res.e = stackEntry{kind: seSpell, ctrl: 0}
	s.execChangeZoneAll(sc.k, &in, false)
	want := []Obj{h[0], h[1], g[0], g[1]}
	if got := libTopOrder(s, 0, 4); !slices.Equal(got, want) {
		t.Fatalf("library top %v, want hand then graveyard %v", got, want)
	}
}

// TestChangeZoneAllShuffleEachOrder: CzShuffleEach shuffles the owner of the
// first card moved first, then the other seat (gorge
// changeZoneAllShuffleLibraries). With seat 0 holding nothing the sweep moves
// only seat 1's cards, so seat 1 shuffles before seat 0; seat 0's own cards
// first make seat 0 shuffle first. The reference runs the same sweep with no
// shuffle flag and shuffles by hand, in each order.
func TestChangeZoneAllShuffleEachOrder(t *testing.T) {
	for _, c := range []struct {
		name   string
		mover  uint8 // the seat holding the cards, and casting (the filter reads your own cards)
		first  uint8 // the seat expected to shuffle first
		second uint8
	}{
		{"seat 1 first", 1, 1, 0},
		{"seat 0 first", 0, 0, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			// the same deterministic scene, built afresh per run (a State is not copyable)
			scene := func(b int16) (*State, *scen, kop.Instr) {
				sc, plains, in := newWeftScen(t)
				sc.grave(plains, c.mover, 3)
				sc.hand(plains, c.mover)
				sc.hand(plains, c.mover)
				sc.s.res.e = stackEntry{kind: seSpell, ctrl: c.mover}
				in.B = kop.Const(int(b))
				return sc.s, sc, in
			}
			// ref sweeps with no shuffle flag, then shuffles by hand in the given order
			ref := func(order [2]uint8) *State {
				r, sc, in := scene(kop.CzHandGrave)
				r.execChangeZoneAll(sc.k, &in, false)
				r.shuffleLib(order[0])
				r.shuffleLib(order[1])
				return r
			}
			want, other := ref([2]uint8{c.first, c.second}), ref([2]uint8{c.second, c.first})
			s, sc, in := scene(kop.CzHandGrave | kop.CzShuffleEach)
			s.execChangeZoneAll(sc.k, &in, false)
			for p := uint8(0); p < 2; p++ {
				if got, w := czLibAll(s, p), czLibAll(want, p); !slices.Equal(got, w) {
					t.Errorf("seat %d library %v, want %v", p, got, w)
				}
			}
			if slices.Equal(czLibAll(other, 0), czLibAll(want, 0)) && slices.Equal(czLibAll(other, 1), czLibAll(want, 1)) {
				t.Fatal("the two shuffle orders leave the same libraries: the test cannot tell them apart")
			}
		})
	}
}
