//go:build dirac_mp

package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/vanilla"
	"github.com/adams-shaun/gorge/engine/rules"
)

// TestInitNFourSeats: four 40-card decks deal each seat its own library and
// opening hand, every object owned by the seat whose deck it came from, laid
// out deck by deck; the toss is gorge's draw over four seats
// (rules.TossWinner), and the mulligan order runs in turn order from it.
func TestInitNFourSeats(t *testing.T) {
	const n = 4
	for seed := uint64(0); seed < 100; seed++ {
		var decks [MaxSeats][]CardID
		for p := 0; p < n; p++ {
			decks[p] = vanilla.Decks[(int(seed)+p)%5]
		}
		s := new(State)
		s.InitN(vanilla.Table, decks, n, seed, Config{Mulligans: 1})
		if s.nSeats() != n {
			t.Fatalf("seed %d: nSeats %d", seed, s.nSeats())
		}
		if want := rules.TossWinner(seed, n); int(s.startPl) != want {
			t.Fatalf("seed %d: toss %d, gorge %d", seed, s.startPl, want)
		}
		for i := uint8(0); i < n; i++ {
			if s.mullOrder[i] != (s.startPl+i)%n {
				t.Fatalf("seed %d: mullOrder %v from %d", seed, s.mullOrder, s.startPl)
			}
		}
		base := 0
		for p := 0; p < n; p++ {
			pl := &s.pl[p]
			if pl.handN != openingHand || int(pl.libHi-pl.libLo) != len(decks[p])-openingHand {
				t.Fatalf("seed %d seat %d: hand %d library %d", seed, p, pl.handN, pl.libHi-pl.libLo)
			}
			var want, got [256]int
			for _, c := range decks[p] {
				want[c]++
			}
			zone := append(append([]Obj(nil), pl.lib[pl.libLo:pl.libHi]...), pl.hand[:pl.handN]...)
			for _, o := range zone {
				if int(o) < base || int(o) >= base+len(decks[p]) {
					t.Fatalf("seed %d seat %d: object %d outside its deck's block", seed, p, o)
				}
				if s.objs[o].owner() != uint8(p) {
					t.Fatalf("seed %d seat %d: object %d owner %d", seed, p, o, s.objs[o].owner())
				}
				got[s.objs[o].card]++
			}
			if got != want {
				t.Fatalf("seed %d seat %d: library and hand are not its deck", seed, p)
			}
			base += len(decks[p])
		}
	}
}
