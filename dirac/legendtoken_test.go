package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestLegendAskBeforeTokenCeases: gorge's state-based pass poses the
// legend rule's keep ask before its other actions apply, so a token
// exiled by the effect that made the duplicate is still in exile at the
// ask (Abyssal Harvester's copy of Zul Ashur exiling the older Nightmare
// copy, pair 3630); it ceases once the answer applies.
func TestLegendAskBeforeTokenCeases(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Giada, Font of Hope", Count: 2}, {Name: "Llanowar Elves", Count: 1}, {Name: "Plains", Count: 8}})
	sc.perm(id["Giada, Font of Hope"], 0)
	sc.perm(id["Giada, Font of Hope"], 0)
	tok := sc.obj(id["Llanowar Elves"], 0)
	sc.s.objs[tok].of |= ofToken
	sc.s.place(tok, kop.ZoneExile, false)
	if sc.s.objs[tok].zone != kop.ZoneExile {
		t.Skip("the token ceased on placement")
	}
	if !sc.s.sba(sc.k) || sc.s.ctl != ctlLegend {
		t.Fatalf("no legend ask (ctl %d)", sc.s.ctl)
	}
	if z := sc.s.objs[tok].zone; z != kop.ZoneExile {
		t.Fatalf("the exiled token is in zone %d at the legend ask, want exile", z)
	}
}

// TestLegendRuleGrantedLegendary: a layer-4 override static that grants the
// Legendary supertype makes otherwise nonlegendary permanents subject to the
// legend rule (CR 205.4d, 704.5j). typesAt is the source of truth, so the
// candidate scan cannot rely on the printed-types legM mask.
func TestLegendRuleGrantedLegendary(t *testing.T) {
	l := newL7(t)
	idx := l.override(kop.Override{AddTypes: kop.TLegendary})
	l.addStatic("Savannah Lions", kop.Static{Kind: kop.StatOverride,
		Filter: l.filter(kop.Filter{}), Override: uint16(idx + 1)})
	l.start(t)
	a := l.perm("Ambush Wolf")
	b := l.perm("Ambush Wolf")
	l.perm("Savannah Lions") // the global override source
	for _, o := range []Obj{a, b} {
		if ty := l.sc.s.typesAt(l.sc.k, l.sc.slot(o)); ty&kop.TLegendary == 0 {
			t.Fatalf("override did not grant Legendary: types %#x", ty)
		}
	}
	d := l.sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChLegend || d.N != 2 {
		t.Fatalf("legend decision kind=%d sub=%d n=%d, want the keep ask over 2", d.Kind, d.Sub, d.N)
	}
	l.sc.s.Apply(MakeAction(AkChoose, TCode(b), 0, 0))
	if l.sc.onBF(a) || !l.sc.onBF(b) {
		t.Fatalf("legend rule kept the wrong one: a=%v b=%v", l.sc.onBF(a), l.sc.onBF(b))
	}
}

// legendCasualtyScen: two Giada under player 0, plus a Llanowar Elves, all
// on the battlefield; the Elves (and, with dupe, the second Giada) carry
// enough -1/-1 counters to be a toughness-0 casualty in the same pass.
func legendCasualtyScen(t *testing.T) (sc *scen, g1, g2, elves Obj) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Giada, Font of Hope", Count: 2}, {Name: "Llanowar Elves", Count: 1}, {Name: "Plains", Count: 8}})
	g1 = sc.perm(id["Giada, Font of Hope"], 0)
	g2 = sc.perm(id["Giada, Font of Hope"], 0)
	elves = sc.perm(id["Llanowar Elves"], 0)
	return
}

// TestLegendAskWithCreatureCasualty: gorge parks the pass's creature
// casualties and asks the legend keep choice first; the answer applies the
// casualties, then the non-kept legend (CR 704.3, 704.5j).
func TestLegendAskWithCreatureCasualty(t *testing.T) {
	sc, g1, g2, elves := legendCasualtyScen(t)
	sc.s.perms[sc.slot(elves)].setCtrN(sc.s, int(kop.CtrM1M1), 2)
	if !sc.s.sba(sc.k) || sc.s.ctl != ctlLegend {
		t.Fatalf("no legend ask (ctl %d)", sc.s.ctl)
	}
	if !sc.onBF(elves) {
		t.Fatal("the casualty died before the legend ask")
	}
	sc.s.Apply(MakeAction(AkChoose, TCode(g1), 0, 0))
	if sc.s.ctl == ctlLegend {
		t.Fatal("still asking")
	}
	if !sc.onBF(g1) || sc.onBF(g2) || sc.onBF(elves) {
		t.Fatalf("after the answer: kept %v, non-kept %v, casualty %v on the battlefield", sc.onBF(g1), sc.onBF(g2), sc.onBF(elves))
	}
	for _, o := range []Obj{g2, elves} {
		if z := sc.s.objs[o].zone; z != kop.ZoneGraveyard {
			t.Fatalf("obj %d in zone %d, want graveyard", o, z)
		}
	}
	if sc.s.legDie.any() {
		t.Fatal("parked casualties not cleared")
	}
}

// TestLegendAskCasualtyIsDuplicate: the casualty is the non-kept duplicate
// itself; it leaves once (as a legend), the kept Giada stays.
func TestLegendAskCasualtyIsDuplicate(t *testing.T) {
	for _, keepFirst := range []bool{true, false} {
		sc, g1, g2, _ := legendCasualtyScen(t)
		sc.s.perms[sc.slot(g2)].setCtrN(sc.s, int(kop.CtrM1M1), 40) // toughness <= 0
		if !sc.s.sba(sc.k) || sc.s.ctl != ctlLegend {
			t.Fatalf("no legend ask (ctl %d)", sc.s.ctl)
		}
		if !sc.onBF(g2) {
			t.Fatal("the casualty died before the legend ask")
		}
		keep, gone := g1, g2
		if !keepFirst {
			keep, gone = g2, g1 // the kept member is itself the casualty: it still dies
		}
		sc.s.Apply(MakeAction(AkChoose, TCode(keep), 0, 0))
		if keepFirst {
			if !sc.onBF(g1) || sc.onBF(gone) || sc.s.objs[gone].zone != kop.ZoneGraveyard {
				t.Fatalf("keep first: kept %v, casualty %v (zone %d)", sc.onBF(g1), sc.onBF(gone), sc.s.objs[gone].zone)
			}
		} else if sc.onBF(g2) || sc.onBF(g1) {
			t.Fatalf("keep casualty: g1 %v g2 %v still on the battlefield", sc.onBF(g1), sc.onBF(g2))
		}
		n := 0
		for _, o := range []Obj{g1, g2} {
			if sc.s.objs[o].zone == kop.ZoneGraveyard {
				n++
			}
		}
		want := 1
		if !keepFirst {
			want = 2
		}
		if n != want {
			t.Fatalf("keepFirst=%v: %d Giada in the graveyard, want %d", keepFirst, n, want)
		}
	}
}
