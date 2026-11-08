package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// townRestrScen: the town pool with two test cards appended, a Test Grove
// ("{T}: Add {G}. Spend this mana only to cast a creature spell") and a Test
// Dual ("{T}: Add {W} or {G}"). p0 has Starting Town, the Dual and the Grove,
// whose {G} floats restricted (State.rb), and a cast of Grizzly Bears is being
// built, so the payment may spend it (CR 106.6). The pool is empty.
func townRestrScen(t *testing.T) (sc *scen, town, dual Slot) {
	t.Helper()
	set := cardsettest.Set(t, "town")
	tbl0, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	tbl := *tbl0
	tbl.Cards = append([]kop.CardDef(nil), tbl0.Cards...)
	tbl.Filters = append([]kop.Filter(nil), tbl0.Filters...)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	spendOn := kop.FilterID(len(tbl.Filters) - 1)
	id := func(name string) CardID {
		i, ok := set.Lookup(name)
		if !ok {
			t.Fatalf("%s not in the town pool", name)
		}
		return CardID(set.PoolID(i))
	}
	grove := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Grove", Types: kop.TArtifact, Abilities: []kop.Ability{
		{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColG, ManaN: 1, ManaRestrict: spendOn},
	}})
	dualID := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Dual", Types: kop.TLand, Abilities: []kop.Ability{
		{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColW | kop.ColG, ManaN: 1},
	}})
	sc = newScenTableLib(t, &tbl, id("Forest"), Config{NoElide: true})
	s := sc.s
	town = sc.slot(sc.perm(id("Starting Town"), 0))
	dual = sc.slot(sc.perm(dualID, 0))
	gr := sc.perm(grove, 0)
	rab := &sc.k.t.Cards[grove].Abilities[sc.k.gen.restr(grove)]
	if !s.floatRestrAb(sc.k, 0, sc.slot(gr), rab, int(kop.G), 1) {
		t.Fatal("precondition: the Grove's {G} did not float restricted")
	}
	*s.bld() = build{e: stackEntry{kind: seSpell, card: id("Grizzly Bears"), ctrl: 0, obj: gr}}
	s.ctl = ctlBuild
	if s.restrPoolAdmits(sc.k, 0) == 0 || s.restrOf(0, int(kop.G)) != 1 {
		t.Fatalf("precondition: restricted %v not admitted for Grizzly Bears", s.rb[0])
	}
	if s.pl[0].pool != ([kop.NumManaKinds]uint8{}) {
		t.Fatalf("precondition: pool %v, want empty", s.pl[0].pool)
	}
	return sc, town, dual
}

// TestStartingTownWindowCountsRestrictedManaOnce: Starting Town's window answer
// (windowCol's painland arm) prices the floating restricted mana the payment may
// spend exactly once (CR 106.6). With the pool {G}{W} and {2}{G}{G} owed, the
// plain {G} and the restricted {G} cover both pips and the pool is one generic
// short, so Starting Town answers {C}. Counting the restricted {G} twice (once in
// the owed cost, again in windowUseful's pool) left no owed colour with the clamp
// decided, and the answer was index 8, past the pool.
func TestStartingTownWindowCountsRestrictedManaOnce(t *testing.T) {
	sc, town, _ := townRestrScen(t)
	s := sc.s
	if !sc.k.painMana {
		t.Fatal("precondition: Starting Town's painful ability is not modelled (k.painMana)")
	}
	s.pl[0].pool = [kop.NumManaKinds]uint8{kop.G: 1, kop.W: 1}
	cost := kop.ManaCost{Generic: 2, Pips: [kop.NumManaKinds]uint8{kop.G: 2}}
	pc := s.windowGrantPC(sc.k, town)
	if pc.painCol != kop.ColAny {
		t.Fatalf("precondition: Starting Town's painCol %v, want any colour", pc.painCol)
	}
	if col := s.windowCol(sc.k, 0, &cost, town, pc, false, false); col != kop.C {
		t.Fatalf("Starting Town answers %d, want {C} (%d)", col, kop.C)
	}
}

// TestTapXWindowColourCountsRestrictedMana: a tap-another window source's colour
// (windowTapXCol, as windowDry's) measures the owed colours against the cost less
// the floating restricted mana, like the paying window's (CR 106.6). The
// restricted {G} covers {G}, so nothing is owed and the Dual answers its lowest
// colour {W}; a raw-cost read would answer {G}.
func TestTapXWindowColourCountsRestrictedMana(t *testing.T) {
	sc, _, dual := townRestrScen(t)
	cost := kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.G: 1}}
	if col := sc.s.windowTapXCol(sc.k, 0, &cost, dual, false, false); col != kop.W {
		t.Fatalf("the Dual answers %d, want {W} (%d)", col, kop.W)
	}
	if col := sc.s.windowTapXCol(sc.k, 0, &kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.G: 2}}, dual, false, false); col != kop.G {
		t.Fatalf("precondition: with {G}{G} owed the Dual answers %d, want {G} (%d)", col, kop.G)
	}
}
