package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// painReflectScen: seat 0 holds an Adarkar Wastes ("{T}: Add {C}" beside the
// painful "{T}: Add {W} or {U}. Deals 1 damage to you") and also a Mana
// Flare-shaped reflect carrier (also seat 0), whose reflected extra (CR 106.12a)
// adds one mana of the type any player's land tap produced. gorge's planner folds the extra into the pick, so
// the land is one tap of two mana of ONE type: {C}{C}, {W}{W} or {U}{U}.
func painReflectScen(t *testing.T) (*scen, Slot) {
	t.Helper()
	set := cardsettest.Set(t, "pain")
	deck := []cardset.Entry{{Name: "Wastes", Count: 8}, {Name: "Adarkar Wastes", Count: 3}}
	tbl, da, _, err := set.Matchup(deck, nil)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	tbl.Filters = append(slices.Clone(tbl.Filters), kop.Filter{Ctrl: kop.CtrlYou})
	mine := kop.FilterID(len(tbl.Filters) - 1)
	flare := CardID(len(tbl.Cards))
	tbl.Cards = append(slices.Clone(tbl.Cards), kop.CardDef{Name: "Test Flare", Types: kop.TEnchantment, Abilities: []kop.Ability{
		{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatTapExtra, B: 1, StFlags: kop.StTapReflect, Filter: mine}},
	}})
	sc := newScenTableLib(t, tbl, id["Wastes"], Config{NoElide: true})
	sc.perm(flare, 0)
	land := sc.perm(id["Adarkar Wastes"], 0)
	if !sc.k.gen.anyTapExtra {
		t.Fatal("precondition: the table has no tap-extra carrier")
	}
	if pc := sc.s.manaPCGen(sc.k, sc.slot(land)); pc.painCol == 0 || pc.painOnly || srcMask(pc) != 1<<kop.C {
		t.Fatalf("precondition: Adarkar Wastes is not a painland with a {C} ability (painCol %b painOnly %v)", pc.painCol, pc.painOnly)
	}
	return sc, sc.slot(land)
}

// TestPainReflectedPickIsOneType: the census prices the painland's tap beside
// a foreign Mana Flare as two mana of one type, with no capacity stall: {W}{W},
// {U}{U} and {C}{C} are payable from it, {W}{U} and {W}{C} are not.
func TestPainReflectedPickIsOneType(t *testing.T) {
	sc, _ := painReflectScen(t)
	var a avail
	sc.s.availFor(sc.k, 0, &a)
	if sc.s.Stalled() {
		t.Fatalf("the census stalled: %s", StallName(sc.s.StallReason()))
	}
	if a.total != 2 {
		t.Fatalf("total = %d, want 2 (one tap, its mana and the reflected extra)", a.total)
	}
	pay := func(c1, c2 int) bool {
		var cost kop.ManaCost
		cost.Pips[c1]++
		cost.Pips[c2]++
		return a.affords(&cost, pipMaskOf(&cost), 0)
	}
	for _, c := range [][2]int{{kop.W, kop.W}, {kop.U, kop.U}, {kop.C, kop.C}} {
		if !pay(c[0], c[1]) {
			t.Errorf("%d%d not payable from the painland's one tap", c[0], c[1])
		}
	}
	for _, c := range [][2]int{{kop.W, kop.U}, {kop.W, kop.C}, {kop.U, kop.C}} {
		if pay(c[0], c[1]) {
			t.Errorf("%d%d payable from one tap of one type", c[0], c[1])
		}
	}
}

// TestPainReflectedPickRemoves: a cost that taps the painland takes the whole
// unit out of the census, its one-type entry included.
func TestPainReflectedPickRemoves(t *testing.T) {
	sc, sl := painReflectScen(t)
	var a avail
	sc.s.availFor(sc.k, 0, &a)
	if a.total != 2 || a.nOne != 1 {
		t.Fatalf("precondition: total %d nOne %d, want 2 and 1", a.total, a.nOne)
	}
	sc.s.removeSrc(sc.k, &a, sl, sc.s.manaPCGen(sc.k, sl))
	if a.total != 0 || a.nOne != 0 {
		t.Fatalf("after removeSrc: total %d nOne %d, want 0 and 0", a.total, a.nOne)
	}
}
