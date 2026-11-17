package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestExileCounterRegionSizedForWriters: the cold exile-counter region is
// paid only by a table whose cards include a dies-to-exile replacement that
// leaves a counter. A counterless ExileOnLeave table (a reader with no
// writer) sizes it 0, and its prep carries no useExileCtr bit.
func TestExileCounterRegionSizedForWriters(t *testing.T) {
	for _, tc := range []struct {
		name    string
		onLeave bool
		writer  bool
		want    int
	}{
		{"no ExileOnLeave", false, false, 0},
		{"counterless ExileOnLeave", true, false, 0},
		{"counter writer", true, true, MaxObj},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := &genPrep{}
			if tc.onLeave {
				g.exileOnLeave = make([]kop.FilterID, 1)
				if tc.writer {
					g.uses.set(useExileCtr)
				}
			}
			if n := exileCtrNeed(g); n != tc.want {
				t.Fatalf("exileCtrNeed sizes %d, want %d", n, tc.want)
			}
		})
	}

	// the store's own pool (Dauthi Voidwalker, Draugr Necromancer) is a
	// writer table: its prep sets the bit and its state pays the region
	sc, _ := exileCtrScen(t)
	if !sc.k.gen.uses.has(useExileCtr) {
		t.Fatal("the exileonleavectr pool's prep has no useExileCtr bit")
	}
	if n := len(coldExCtr.of(sc.s)); n != MaxObj {
		t.Fatalf("the writer pool's cold region is %d records, want %d", n, MaxObj)
	}
}
