package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTwoPilesOneCardSkipsSplit: gorge poses no separation ask for a
// one-card set (a decision nobody could answer differently; pile A is that
// card, effects/twopiles.go) and goes straight to the opponent's pile
// pick. Curator of Destinies entering with one card left in the library
// (pair 1004): before, the kernel asked its controller [done, the card].
func TestTwoPilesOneCardSkipsSplit(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Curator of Destinies", Count: 1}, {Name: "Island", Count: 8}})
	pl := &sc.s.pl[0]
	for pl.libHi-pl.libLo > 1 {
		o := pl.lib[pl.libLo]
		sc.s.detach(o)
		sc.s.place(o, kop.ZoneGraveyard, false)
	}
	for i := 0; i < 6; i++ {
		sc.perm(id["Island"], 0)
	}
	cu := sc.hand(id["Curator of Destinies"], 0)
	split, pick := false, false
	sc.stepTo(t, 40, func() bool { return pick }, func(d *Decision) (Action, bool) {
		if d.Kind == DkChoose && d.Sub == ChGenPileSplit {
			split = true
		}
		if d.Kind == DkChoose && d.Sub == ChGenPileChoose {
			if d.Player != 1 {
				t.Fatalf("pile pick posed to p%d", d.Player)
			}
			pick = true
		}
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(cu)) {
				return a, true
			}
		}
		for i := 0; i < int(d.N); i++ {
			if d.Options[i].Kind() == AkPass {
				return d.Options[i], true
			}
		}
		return 0, false
	})
	if !pick {
		t.Fatal("the opponent was never asked to pick a pile")
	}
	if split {
		t.Fatal("a one-card set was posed a separation ask")
	}
}
