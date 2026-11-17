package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestValueTargetGrave: X = a targeted card's toughness (and power) reads a
// graveyard card's printed values when it is not on the battlefield. The
// resolving value used to return 0 for every off-battlefield target with no
// last known information snapshot, so Morbid Bloom ("exile target creature
// card from a graveyard, then create X Saprolings, X = its toughness", CR
// 608.2g/h) exiled a 2/3 and made no tokens.
func TestValueTargetGrave(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Vampire Nighthawk", Count: 1}, {Name: "Swamp", Count: 8}})
	grave := sc.grave(id["Vampire Nighthawk"], 1, 1)[0]
	if z := sc.s.objs[grave].zone; z != kop.ZoneGraveyard {
		t.Fatalf("the target card is in zone %v, want the graveyard", z)
	}
	e := &sc.s.res.e
	e.kind, e.ctrl, e.card = seSpell, 0, id["Vampire Nighthawk"]
	e.tgt[0], e.nt, e.tver[0] = TCode(grave), 1, sc.s.objs[grave].ver
	if p := sc.s.value(sc.k, kop.ValueRef{K: kop.VTargetPower}); p != 2 {
		t.Fatalf("VTargetPower of a graveyard 2/3 card = %d, want 2", p)
	}
	if q := sc.s.value(sc.k, kop.ValueRef{K: kop.VTargetToughness}); q != 3 {
		t.Fatalf("VTargetToughness of a graveyard 2/3 card = %d, want 3", q)
	}

	// A battlefield target still reads its derived P/T, not the printed 2/3.
	live := sc.perm(id["Vampire Nighthawk"], 0)
	sc.s.perms[sc.slot(live)].setCtrN(sc.s, int(kop.CtrP1P1), 1)
	e.tgt[0], e.tver[0] = TCode(live), sc.s.objs[live].ver
	if p := sc.s.value(sc.k, kop.ValueRef{K: kop.VTargetPower}); p != 3 {
		t.Fatalf("VTargetPower of a 2/3 with a +1/+1 counter = %d, want the derived 3", p)
	}
	if q := sc.s.value(sc.k, kop.ValueRef{K: kop.VTargetToughness}); q != 4 {
		t.Fatalf("VTargetToughness of a 2/3 with a +1/+1 counter = %d, want the derived 4", q)
	}
}
