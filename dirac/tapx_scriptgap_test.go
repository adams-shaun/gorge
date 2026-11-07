package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTapXCostWithheldFromEachColourGap: Wanderbrine Trapper's "{1}, {T}, Tap
// another untapped creature you control: Tap target creature an opponent
// controls." is not offered when the only untapped creature the tap part could
// take is the each-colour source its {1} would also tap (CR 601.2g-h; gorge's
// planEachReservation withholds the tap candidate from the mana). The card's
// mana is a scripted gap the plain walk does not count, so the offer has to
// read the gap's units as the same permanent the tap cost takes.
func TestTapXCostWithheldFromEachColourGap(t *testing.T) {
	for _, c := range []struct {
		name  string
		extra []string
		want  bool
	}{
		{"the each-colour source is the only other creature and the only mana", nil, false},
		{"an untapped land pays the mana", []string{"Forest"}, true},
		{"a second creature pays the tap", []string{"Changeling Wayfinder"}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			names := append([]string{"Wanderbrine Trapper", "Bloom Tender", "Forest", "Plains", "Changeling Wayfinder"}, c.extra...)
			sc, id := synthScen(t, "std-ecl", names...)
			trap := sc.perm(id["Wanderbrine Trapper"], 0)
			bloom := sc.perm(id["Bloom Tender"], 0)
			for _, o := range []Obj{sc.perm(id["Forest"], 0), sc.perm(id["Plains"], 0), sc.perm(id["Plains"], 0)} {
				sc.s.perms[sc.slot(o)].flags |= pfTapped
			}
			sc.perm(id["Changeling Wayfinder"], 1)
			for _, n := range c.extra {
				sc.perm(id[n], 0)
			}
			// Preconditions: the source has the tapX + {T} cost and {1}, the
			// each-colour source is an untapped gap, the lands are spent and
			// the pool is empty, so only the gap's mana could pay the cost.
			a := &sc.k.t.Cards[sc.s.objs[trap].card].Abilities[0]
			if a.Cost.Flags&kop.CostTap == 0 || a.Cost.Flags&kop.CostTapX == 0 || a.Cost.Mana.MV() != 1 {
				t.Fatalf("Trapper ability cost flags %d mana %v, want {1}, {{T}} and TapX", a.Cost.Flags, a.Cost.Mana)
			}
			if bl := sc.slot(bloom); !sc.s.scriptGaps(sc.k, 0).has(bl) || sc.s.perms[bl].flags&pfTapped != 0 {
				t.Fatalf("Bloom Tender slot %d not an untapped each-colour gap", bl)
			}
			for col, n := range sc.s.pl[0].pool {
				if n != 0 {
					t.Fatalf("pool[%d] = %d, want empty", col, n)
				}
			}
			if got := offersAb(sc.s, trap, 0); got != c.want {
				t.Errorf("activation offered %v, want %v with extra %v", got, c.want, c.extra)
			}
		})
	}
}
