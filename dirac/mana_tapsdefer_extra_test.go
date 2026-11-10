package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTapsDeferFixedColourGapCarriesTheTapExtra: a deferred one-mana fixed
// colour tap (the Core rewritten to "{T}: Add {G}", the Llanowar Elves under
// Cryoshatter shape) beside a StatTapExtra carrier (Badgermole Cub's
// static, grafted onto the Birds: "whenever you tap a creature for mana, add
// an additional {G}") is a gap whose census unit and float carry the extra,
// as gorge's relaxed alternative does (potential_plan.go potentialTapExtra).
func TestTapsDeferFixedColourGapCarriesTheTapExtra(t *testing.T) {
	// Badgermole Cub's static, built by hand (filters are table-local, and
	// the Core is no creature: an open "permanent you control" filter)
	carrier := kop.Ability{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatTapExtra, A: kop.G, B: 1}}
	tbl, id := tapsDeferShapeTable(t, func(tbl *kop.Table, id map[string]CardID) {
		rewriteMana(tbl, id["Firdoch Core"], func(a *kop.Ability) { a.ManaColors = kop.ColG })
		ab := carrier
		ab.Static.Filter = kop.FilterID(len(tbl.Filters))
		tbl.Filters = append(append([]kop.Filter(nil), tbl.Filters...), kop.Filter{Ctrl: kop.CtrlYou})
		b := &tbl.Cards[id["Birds of Paradise"]]
		b.Abilities = append(append([]kop.Ability(nil), b.Abilities...), ab)
	})
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	core := sc.perm(id["Firdoch Core"], 0)
	sc.perm(id["Judge of Currents"], 0)
	sc.perm(id["Birds of Paradise"], 0)
	if !sc.k.gen.anyTapExtra {
		t.Fatal("precondition: the table has no tap-extra carrier")
	}
	sl := sc.slot(core)
	if !sc.s.scriptTapsDefer(sc.k, 0).has(sl) {
		t.Fatal("precondition: the {G} Core is no script gap")
	}
	xv, hit := sc.s.srcExtraHit(sc.k, sl, sc.s.manaPCGen(sc.k, sl))
	if !hit || xv[carrier.Static.A] != uint8(carrier.Static.B) {
		t.Fatalf("precondition: the carrier reaches the Core with extra %v (hit %v), want %d of colour %d", xv, hit, carrier.Static.B, carrier.Static.A)
	}
	want := 1 + int(carrier.Static.B)
	var a avail
	sc.s.addTapsDeferUnits(sc.k, sl, &a)
	if a.total != want {
		t.Fatalf("gap census unit %d, want %d (its {G} and the extra)", a.total, want)
	}
	var gaps avail
	sc.s.scriptGapAvail(sc.k, 0, sl, &gaps)
	if gaps.total != want {
		t.Fatalf("scriptGapAvail %d, want %d", gaps.total, want)
	}
	before := sc.s.pl[0].pool
	sc.s.tapDeferFloat(sc.k, 0, sl, 0)
	got := 0
	for c := range before {
		got += int(sc.s.pl[0].pool[c]) - int(before[c])
	}
	if got != want {
		t.Fatalf("the gap's float adds %d mana (pool %v), want %d", got, sc.s.pl[0].pool, want)
	}
}
